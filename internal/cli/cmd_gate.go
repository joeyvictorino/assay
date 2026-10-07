package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("gate", "check a target URL against a signed scope document", runGate)
}

// runGate: assay gate check --scope FILE --trust DIR --target URL [--method M]
//
// Exit 0 when allowed, 2 when denied or on any configuration error (a gate
// denial is a tool/config error, never a pass).
func runGate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "check" {
		fmt.Fprintln(stderr, "usage: assay gate check --scope FILE --trust DIR --target URL [--method GET] [--unsafe-skip-signature]")
		return ExitError
	}
	fs := flag.NewFlagSet("gate check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	scopePath := fs.String("scope", "", "signed scope document (YAML or JSON)")
	trustDir := fs.String("trust", "", "trust store directory with scope signing keys")
	target := fs.String("target", "", "target URL to check")
	method := fs.String("method", "GET", "HTTP method to check")
	unsafeSkip := fs.Bool("unsafe-skip-signature", false, "DEVELOPMENT ONLY: skip signature verification (requires ASSAY_DEV=1)")
	if err := fs.Parse(args[1:]); err != nil {
		return ExitError
	}
	if *target == "" {
		return fail(stderr, "gate", errors.New("--target is required"))
	}
	g, err := loadGate(*scopePath, *trustDir, *unsafeSkip, stderr)
	if err != nil {
		return fail(stderr, "gate", err)
	}
	d := g.Allow(context.Background(), *target)
	if d.Effect == model.EffectAllow {
		if md := g.AllowMethod(*method); md.Effect != model.EffectAllow {
			d = md
		}
	}
	out := struct {
		Target      string         `json:"target"`
		Method      string         `json:"method"`
		Fingerprint string         `json:"scope_fingerprint"`
		KeyID       string         `json:"scope_key_id"`
		Decision    model.Decision `json:"decision"`
	}{*target, *method, g.Fingerprint(), g.KeyID(), d}
	if err := writeJSON("", out, stdout); err != nil {
		return fail(stderr, "gate", err)
	}
	if d.Effect != model.EffectAllow {
		return ExitError
	}
	return ExitPass
}
