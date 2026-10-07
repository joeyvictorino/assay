package cli

import (
	"crypto/ed25519"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/joeyvictorino/assay/internal/scope"
	"github.com/joeyvictorino/assay/internal/toolsig"
)

// Lead-owned: wires the trust store (S1) into the scope gate commands (S3)
// and signs scope documents with the same Ed25519 key as tool manifests.
func init() {
	ScopeVerifier = func(trustDir string) (scope.Verifier, error) {
		ts, err := toolsig.LoadTrustStore(trustDir)
		if err != nil {
			return nil, err
		}
		return ts.VerifyDocument, nil
	}
	Register("scope", "sign or verify a scope document (sign|verify)", runScope)
}

func runScope(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: assay scope sign --in FILE --out FILE --key FILE | assay scope verify --in FILE --trust DIR")
		return ExitError
	}
	switch args[0] {
	case "sign":
		return scopeSign(args[1:], stdout, stderr)
	case "verify":
		return scopeVerify(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "assay scope: unknown subcommand %q\n", args[0])
	return ExitError
}

func scopeSign(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scope sign", flag.ContinueOnError)
	fs.SetOutput(stderr)
	in := fs.String("in", "", "unsigned scope YAML")
	out := fs.String("out", "", "signed scope YAML to write")
	key := fs.String("key", "", "Ed25519 private key file (from assay tools keygen)")
	if err := fs.Parse(args); err != nil || *in == "" || *out == "" || *key == "" {
		fmt.Fprintln(stderr, "usage: assay scope sign --in FILE --out FILE --key FILE")
		return ExitError
	}
	raw, err := os.ReadFile(*in)
	if err != nil {
		return fail(stderr, "scope sign", err)
	}
	doc := map[string]any{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fail(stderr, "scope sign", err)
	}
	kb, err := os.ReadFile(*key)
	if err != nil {
		return fail(stderr, "scope sign", err)
	}
	priv, err := toolsig.ParsePrivateKey(string(kb))
	if err != nil {
		return fail(stderr, "scope sign", err)
	}
	signed, err := toolsig.SignDocument(doc, priv, time.Now())
	if err != nil {
		return fail(stderr, "scope sign", err)
	}
	// Prove the signed document also loads through the gate before writing.
	ts, err := toolsig.NewTrustStore(map[string]ed25519.PublicKey{"self": priv.Public().(ed25519.PublicKey)})
	if err != nil {
		return fail(stderr, "scope sign", err)
	}
	data, err := yaml.Marshal(signed)
	if err != nil {
		return fail(stderr, "scope sign", err)
	}
	if _, err := scope.Parse(data, time.Now(), ts.VerifyDocument); err != nil {
		return fail(stderr, "scope sign (self-check)", err)
	}
	header := "# Signed scope document. Do not edit by hand; re-run `assay scope sign`.\n"
	if err := os.WriteFile(*out, append([]byte(header), data...), 0o644); err != nil {
		return fail(stderr, "scope sign", err)
	}
	fmt.Fprintf(stdout, "signed %s -> %s (signer %s)\n", *in, *out, signed[toolsig.FieldSigner])
	return ExitPass
}

func scopeVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("scope verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	in := fs.String("in", "", "signed scope YAML")
	trust := fs.String("trust", "trust/keys", "trust store directory")
	if err := fs.Parse(args); err != nil || *in == "" {
		fmt.Fprintln(stderr, "usage: assay scope verify --in FILE [--trust DIR]")
		return ExitError
	}
	v, err := ScopeVerifier(*trust)
	if err != nil {
		return fail(stderr, "scope verify", err)
	}
	g, err := scope.Load(*in, time.Now(), v)
	if err != nil {
		return fail(stderr, "scope verify", err)
	}
	fmt.Fprintf(stdout, "scope OK: fingerprint %s\n", g.Fingerprint())
	return ExitPass
}
