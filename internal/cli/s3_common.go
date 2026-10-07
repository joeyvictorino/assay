package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/scope"
)

// ScopeVerifier builds the scope-signature verifier from a trust directory.
//
// LEAD: internal/toolsig (S1) owns signature verification. Set this from the
// wiring code (for example in an init in a file S1 owns) so `assay gate`,
// `assay validate` and friends verify real signatures. Until it is set, the
// commands refuse to load a scope unless --unsafe-skip-signature is given
// together with ASSAY_DEV=1.
var ScopeVerifier func(trustDir string) (scope.Verifier, error)

const unsafeWarning = `
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
!!  SCOPE SIGNATURE CHECK SKIPPED (--unsafe-skip-signature, ASSAY_DEV=1)   !!
!!  The scope document is NOT verified. Development use only. Never in CI.  !!
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
`

// loadGate loads and verifies a scope document for the S3 commands.
func loadGate(scopePath, trustDir string, unsafeSkip bool, stderr io.Writer) (*scope.Gate, error) {
	if scopePath == "" {
		return nil, errors.New("--scope is required")
	}
	var verify scope.Verifier
	switch {
	case unsafeSkip:
		if os.Getenv("ASSAY_DEV") != "1" {
			return nil, errors.New("--unsafe-skip-signature requires ASSAY_DEV=1 in the environment; refusing")
		}
		fmt.Fprint(stderr, unsafeWarning)
		verify = func(map[string]any) (string, string, error) { return "UNSAFE-DEV-SKIP", "", nil }
	case ScopeVerifier != nil:
		if trustDir == "" {
			return nil, errors.New("--trust is required")
		}
		v, err := ScopeVerifier(trustDir)
		if err != nil {
			return nil, fmt.Errorf("trust store: %w", err)
		}
		verify = v
	default:
		return nil, errors.New("scope signature verifier is not wired in this build (" + scope.CodeVerifierMissing + "); for local development only, pass --unsafe-skip-signature with ASSAY_DEV=1")
	}
	g, err := scope.Load(scopePath, time.Now().UTC(), verify)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// readFindings reads a JSON array of findings from path ("-" for stdin).
func readFindings(path string, stdin io.Reader) ([]model.Finding, error) {
	if path == "" {
		return nil, errors.New("--findings is required")
	}
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	var fs []model.Finding
	if err := json.Unmarshal(data, &fs); err != nil {
		// Also accept {"findings": [...]}.
		var wrapped struct {
			Findings []model.Finding `json:"findings"`
		}
		if err2 := json.Unmarshal(data, &wrapped); err2 != nil || wrapped.Findings == nil {
			return nil, fmt.Errorf("findings: %w", err)
		}
		fs = wrapped.Findings
	}
	return fs, nil
}

// writeJSON writes v as indented JSON to path ("" or "-" for stdout).
func writeJSON(path string, v any, stdout io.Writer) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if path == "" || path == "-" {
		_, err = stdout.Write(b)
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// writeText writes s to path ("" or "-" for stdout).
func writeText(path, s string, stdout io.Writer) error {
	if path == "" || path == "-" {
		_, err := io.WriteString(stdout, s)
		return err
	}
	return os.WriteFile(path, []byte(s), 0o644)
}

func fail(stderr io.Writer, cmd string, err error) int {
	fmt.Fprintf(stderr, "assay %s: %v\n", cmd, err)
	return ExitError
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
