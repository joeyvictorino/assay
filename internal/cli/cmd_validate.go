package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/labs"
	"github.com/joeyvictorino/assay/internal/labs/dvwa"
	"github.com/joeyvictorino/assay/internal/labs/juiceshop"
	"github.com/joeyvictorino/assay/internal/labs/syntheticops"
	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/validate"
)

func init() {
	Register("validate", "run deterministic checkers over a findings file", runValidate)
}

// runValidate: assay validate --findings FILE --out FILE --scope FILE --trust DIR
//
//	--lab synthetic-ops|juice-shop|dvwa --base-url URL [--lab-dir DIR] [--setup]
//	[--audit FILE] [--run-id ID] [--unsafe-skip-signature]
func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	findingsPath := fs.String("findings", "", "findings JSON file (array of findings, or - for stdin)")
	outPath := fs.String("out", "", "output file (default stdout)")
	scopePath := fs.String("scope", "", "signed scope document")
	trustDir := fs.String("trust", "", "trust store directory")
	labName := fs.String("lab", "", "lab adapter: synthetic-ops, juice-shop or dvwa")
	baseURL := fs.String("base-url", "", "lab base URL (must be inside the scope)")
	labDir := fs.String("lab-dir", "", "lab data directory (default labs/<lab>)")
	setup := fs.Bool("setup", false, "run the lab's Setup before validating")
	auditPath := fs.String("audit", "", "append plaintext JSONL audit records here (default stderr)")
	runID := fs.String("run-id", "", "run id recorded in audit records (default derived from time)")
	timeout := fs.Duration("timeout", 15*time.Second, "per-request timeout")
	unsafeSkip := fs.Bool("unsafe-skip-signature", false, "DEVELOPMENT ONLY: skip signature verification (requires ASSAY_DEV=1)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	findings, err := readFindings(*findingsPath, os.Stdin)
	if err != nil {
		return fail(stderr, "validate", err)
	}
	if *baseURL == "" {
		return fail(stderr, "validate", errors.New("--base-url is required"))
	}
	gate, err := loadGate(*scopePath, *trustDir, *unsafeSkip, stderr)
	if err != nil {
		return fail(stderr, "validate", err)
	}
	ctx := context.Background()
	if d := gate.Allow(ctx, *baseURL); d.Effect != model.EffectAllow {
		return fail(stderr, "validate", fmt.Errorf("base url denied by scope: %s: %s", d.Reason, d.Rationale))
	}

	var auditW io.Writer = stderr
	if *auditPath != "" {
		f, err := os.OpenFile(*auditPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return fail(stderr, "validate", err)
		}
		defer f.Close()
		auditW = f
	}
	if *runID == "" {
		*runID = "run-" + time.Now().UTC().Format("20060102T150405Z")
	}
	// LEAD: swap WriterAuditor for internal/audit (S1) when wiring the binary.
	client := &httpx.Client{
		Gate:    gate,
		Auditor: &httpx.WriterAuditor{W: auditW},
		RunID:   *runID,
		Agent:   "validate",
		Timeout: *timeout,
		MaxBody: gate.MaxBodyBytes(),
	}
	lab, env, err := buildLab(*labName, *baseURL, *labDir, client)
	if err != nil {
		return fail(stderr, "validate", err)
	}
	if err := lab.Health(ctx); err != nil {
		return fail(stderr, "validate", fmt.Errorf("lab health: %w", err))
	}
	if *setup {
		if err := lab.Setup(ctx); err != nil {
			return fail(stderr, "validate", fmt.Errorf("lab setup: %w", err))
		}
	}
	out := validate.Validate(ctx, findings, env)
	if err := writeJSON(*outPath, out, stdout); err != nil {
		return fail(stderr, "validate", err)
	}
	counts := map[model.FindingState]int{}
	for _, f := range out {
		counts[f.State]++
	}
	fmt.Fprintf(stderr, "assay validate: %d findings: %d validated, %d refuted, %d theorized, %d declined\n",
		len(out), counts[model.StateValidated], counts[model.StateRefuted], counts[model.StateTheorized], counts[model.StateDeclined])
	return ExitPass
}

// buildLab constructs the adapter and validation environment for a lab name.
func buildLab(name, baseURL, dir string, client *httpx.Client) (labs.Lab, validate.Env, error) {
	switch name {
	case syntheticops.Name:
		if dir == "" {
			dir = filepath.Join("labs", "synthetic-ops")
		}
		lab := syntheticops.New(baseURL, dir, client)
		env := validate.Env{HTTP: client, Lab: lab, Marker: syntheticops.SecretMarker,
			Markers: map[string]string{"traversal-file": "README.md", "traversal-marker": syntheticops.ReadmeMarker}}
		return lab, env, nil
	case juiceshop.Name:
		if dir == "" {
			dir = filepath.Join("labs", "juice-shop")
		}
		lab := juiceshop.New(baseURL, filepath.Join(dir, "accounts.yaml"), filepath.Join(dir, "reference_findings.json"), client)
		return lab, validate.Env{HTTP: client, Lab: lab}, nil
	case dvwa.Name:
		if dir == "" {
			dir = filepath.Join("labs", "dvwa")
		}
		lab := dvwa.New(baseURL, filepath.Join(dir, "reference_findings.json"), client)
		return lab, validate.Env{HTTP: client, Lab: lab}, nil
	case "":
		return nil, validate.Env{}, errors.New("--lab is required (synthetic-ops, juice-shop or dvwa)")
	}
	return nil, validate.Env{}, fmt.Errorf("unknown lab %q", name)
}
