package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/scope"
)

const testScope = `version: "1"
authorized_by: ci
issued_at: "2020-01-01T00:00:00Z"
expires_at: "2099-01-01T00:00:00Z"
rules_of_engagement_sha256: "0000000000000000000000000000000000000000000000000000000000000000"
targets:
  - host: 127.0.0.1
    ports: [%s]
    schemes: [http]
methods: [GET, POST]
rate_limit_per_minute: 600
max_body_bytes: 1048576
signature: {key_id: dev, sig: "x"}
`

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func runS3(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Main(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestGateRefusesWithoutVerifierOrDevFlag(t *testing.T) {
	scopePath := writeTemp(t, "scope.yaml", strings.Replace(testScope, "%s", "9000", 1))
	t.Setenv("ASSAY_DEV", "")
	code, _, errs := runS3(t, "gate", "check", "--scope", scopePath, "--target", "http://127.0.0.1:9000/")
	// With the trust store wired in (cmd_scope.go), an unsigned run without
	// --trust is refused for a missing trust dir; without the wiring it is
	// refused for a missing verifier. Both fail closed with exit 2.
	if code != ExitError || !(strings.Contains(errs, "SCOPE_UNSIGNED_VERIFIER_MISSING") || strings.Contains(errs, "--trust is required")) {
		t.Fatalf("code=%d err=%q", code, errs)
	}
	// --unsafe-skip-signature without ASSAY_DEV=1 is refused too.
	code, _, errs = runS3(t, "gate", "check", "--scope", scopePath, "--target", "http://127.0.0.1:9000/", "--unsafe-skip-signature")
	if code != ExitError || !strings.Contains(errs, "ASSAY_DEV=1") {
		t.Fatalf("code=%d err=%q", code, errs)
	}
	if code, _, _ := runS3(t, "gate"); code != ExitError {
		t.Fatal("missing subcommand")
	}
	if code, _, _ := runS3(t, "gate", "check", "--scope", scopePath); code != ExitError {
		t.Fatal("missing target")
	}
}

func TestGateCheckDevMode(t *testing.T) {
	scopePath := writeTemp(t, "scope.yaml", strings.Replace(testScope, "%s", "9000", 1))
	t.Setenv("ASSAY_DEV", "1")
	code, out, errs := runS3(t, "gate", "check", "--scope", scopePath, "--target", "http://127.0.0.1:9000/api", "--unsafe-skip-signature")
	if code != ExitPass || !strings.Contains(errs, "SIGNATURE CHECK SKIPPED") {
		t.Fatalf("code=%d out=%q err=%q", code, out, errs)
	}
	var res struct {
		Decision model.Decision `json:"decision"`
		KeyID    string         `json:"scope_key_id"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil || res.Decision.Effect != model.EffectAllow || res.KeyID != "UNSAFE-DEV-SKIP" {
		t.Fatalf("%q %v", out, err)
	}
	code, out, _ = runS3(t, "gate", "check", "--scope", scopePath, "--target", "http://evil.test/", "--unsafe-skip-signature")
	if code != ExitError || !strings.Contains(out, "OUT_OF_SCOPE") {
		t.Fatalf("denial: code=%d out=%q", code, out)
	}
	code, out, _ = runS3(t, "gate", "check", "--scope", scopePath, "--target", "http://127.0.0.1:9000/", "--method", "DELETE", "--unsafe-skip-signature")
	if code != ExitError || !strings.Contains(out, "METHOD_NOT_ALLOWED") {
		t.Fatalf("method denial: code=%d out=%q", code, out)
	}
}

func TestGateUsesWiredVerifier(t *testing.T) {
	old := ScopeVerifier
	defer func() { ScopeVerifier = old }()
	ScopeVerifier = func(trustDir string) (scope.Verifier, error) {
		return func(map[string]any) (string, string, error) { return "trust-key-" + filepath.Base(trustDir), "", nil }, nil
	}
	scopePath := writeTemp(t, "scope.yaml", strings.Replace(testScope, "%s", "9000", 1))
	t.Setenv("ASSAY_DEV", "")
	code, out, errs := runS3(t, "gate", "check", "--scope", scopePath, "--trust", "/tmp/keys", "--target", "http://127.0.0.1:9000/")
	if code != ExitPass || !strings.Contains(out, `"trust-key-keys"`) {
		t.Fatalf("code=%d out=%q err=%q", code, out, errs)
	}
	if code, _, errs := runS3(t, "gate", "check", "--scope", scopePath, "--target", "http://127.0.0.1:9000/"); code != ExitError || !strings.Contains(errs, "--trust") {
		t.Fatalf("missing trust dir: %d %q", code, errs)
	}
}

func sampleFindings() []model.Finding {
	return []model.Finding{
		{ID: "F-DC", Lab: "synthetic-ops", Class: model.ClassDefaultCreds, State: model.StateValidated, Severity: model.SevHigh,
			Location: model.Location{Method: "POST", PathTemplate: "/api/login", Param: "password"}},
		{ID: "F-IDOR", Lab: "synthetic-ops", Class: model.ClassIDOR, State: model.StateValidated, Severity: model.SevHigh,
			Location: model.Location{Method: "GET", PathTemplate: "/api/users/{id}", Param: "id"}},
		{ID: "F-SQLI", Lab: "synthetic-ops", Class: model.ClassSQLi, State: model.StateTheorized, Severity: model.SevCritical,
			Location: model.Location{Method: "GET", PathTemplate: "/api/search", Param: "q"}},
	}
}

func TestChainCommand(t *testing.T) {
	b, _ := json.Marshal(sampleFindings())
	in := writeTemp(t, "findings.json", string(b))
	out := filepath.Join(t.TempDir(), "paths.json")
	mm := filepath.Join(t.TempDir(), "paths.mmd")
	code, _, errs := runS3(t, "chain", "--findings", in, "--out", out, "--mermaid", mm)
	if code != ExitPass {
		t.Fatalf("code=%d err=%q", code, errs)
	}
	data, _ := os.ReadFile(out)
	var paths []model.AttackPath
	if err := json.Unmarshal(data, &paths); err != nil || len(paths) == 0 {
		t.Fatalf("%s %v", data, err)
	}
	m, _ := os.ReadFile(mm)
	if !strings.HasPrefix(string(m), "flowchart LR") {
		t.Fatalf("mermaid %q", m)
	}
	if code, _, _ := runS3(t, "chain", "--findings", filepath.Join(t.TempDir(), "nope.json")); code != ExitError {
		t.Fatal("missing file must be an error")
	}
	if code, _, _ := runS3(t, "chain"); code != ExitError {
		t.Fatal("missing --findings must be an error")
	}
}

func TestRemediateCommand(t *testing.T) {
	b, _ := json.Marshal(sampleFindings())
	in := writeTemp(t, "findings.json", string(b))
	out := filepath.Join(t.TempDir(), "rem.json")
	code, _, errs := runS3(t, "remediate", "--findings", in, "--out", out,
		"--ground-truth", "../../labs/synthetic-ops/ground_truth.json", "--fixes-dir", "../../labs/synthetic-ops/fixes")
	if code != ExitPass {
		t.Fatalf("code=%d err=%q", code, errs)
	}
	data, _ := os.ReadFile(out)
	var fs []model.Finding
	if err := json.Unmarshal(data, &fs); err != nil || len(fs) != 3 {
		t.Fatalf("%s %v", data, err)
	}
	if fs[0].ID != "F-SQLI" {
		t.Fatalf("critical first, got %s", fs[0].ID)
	}
	if fs[0].Remediation == nil || fs[0].Remediation.CodeDiff == "" || !strings.Contains(fs[0].Remediation.VirtualPatch, "SecRule") {
		t.Fatalf("sqli remediation %+v", fs[0].Remediation)
	}
	for _, f := range fs {
		if f.Remediation == nil || f.Remediation.ConfigChange == "" {
			t.Errorf("%s: missing remediation", f.ID)
		}
	}
	// Wrapped input shape is accepted.
	wrapped := writeTemp(t, "wrapped.json", `{"findings":`+string(b)+`}`)
	if code, _, errs := runS3(t, "remediate", "--findings", wrapped, "--out", out); code != ExitPass {
		t.Fatalf("wrapped: %d %q", code, errs)
	}
	if code, _, _ := runS3(t, "remediate", "--findings", in, "--ground-truth", filepath.Join(t.TempDir(), "x.json")); code != ExitError {
		t.Fatal("bad ground truth must fail")
	}
}

func TestValidateCommandAgainstMimic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.Write([]byte(`{"ok":true}`))
		case "/debug/config":
			w.Write([]byte(`{"k":"SYNTH-SECRET-MARKER-0001"}`))
		default:
			w.Write([]byte("<html>ok</html>"))
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	scopePath := writeTemp(t, "scope.yaml", strings.Replace(testScope, "%s", u.Port(), 1))
	findings := []model.Finding{
		{ID: "F-HDR", Lab: "synthetic-ops", Class: model.ClassSecurityHeaders, State: model.StateTheorized, Severity: model.SevLow,
			Location: model.Location{Method: "GET", PathTemplate: "/"}},
		{ID: "F-INFO", Lab: "synthetic-ops", Class: model.ClassInfoDisclosure, State: model.StateTheorized, Severity: model.SevHigh,
			Location: model.Location{Method: "GET", PathTemplate: "/debug/config"}},
		{ID: "F-CSRF", Lab: "synthetic-ops", Class: model.ClassCSRF, State: model.StateTheorized, Severity: model.SevLow,
			Location: model.Location{Method: "POST", PathTemplate: "/x"}},
	}
	b, _ := json.Marshal(findings)
	in := writeTemp(t, "findings.json", string(b))
	out := filepath.Join(t.TempDir(), "validated.json")
	audit := filepath.Join(t.TempDir(), "audit.jsonl")
	t.Setenv("ASSAY_DEV", "1")
	code, _, errs := runS3(t, "validate", "--findings", in, "--out", out, "--scope", scopePath, "--unsafe-skip-signature",
		"--lab", "synthetic-ops", "--lab-dir", "../../labs/synthetic-ops", "--base-url", srv.URL, "--audit", audit)
	if code != ExitPass {
		t.Fatalf("code=%d err=%q", code, errs)
	}
	data, _ := os.ReadFile(out)
	var got []model.Finding
	if err := json.Unmarshal(data, &got); err != nil || len(got) != 3 {
		t.Fatalf("%s %v", data, err)
	}
	if got[0].State != model.StateValidated || got[1].State != model.StateValidated || got[2].State != model.StateTheorized {
		t.Fatalf("states: %s %s %s", got[0].State, got[1].State, got[2].State)
	}
	al, _ := os.ReadFile(audit)
	if !strings.Contains(string(al), `"kind":"tool_call"`) || !strings.Contains(string(al), `"kind":"gate_decision"`) {
		t.Fatalf("audit log %s", al)
	}
	if strings.Contains(string(al), "SYNTH-SECRET-MARKER") {
		t.Fatal("audit log must not contain response bodies")
	}
	// A base URL outside the scope is refused before any request.
	code, _, errs = runS3(t, "validate", "--findings", in, "--scope", scopePath, "--unsafe-skip-signature",
		"--lab", "synthetic-ops", "--base-url", "http://127.0.0.1:1/")
	if code != ExitError || !strings.Contains(errs, "denied by scope") {
		t.Fatalf("code=%d err=%q", code, errs)
	}
	if code, _, errs := runS3(t, "validate", "--findings", in, "--scope", scopePath, "--unsafe-skip-signature", "--lab", "nope", "--base-url", srv.URL); code != ExitError || !strings.Contains(errs, "unknown lab") {
		t.Fatalf("code=%d err=%q", code, errs)
	}
}
