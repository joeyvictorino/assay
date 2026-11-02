package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/audit"
	"github.com/joeyvictorino/assay/internal/model"
)

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Main(args, &out, &errb)
	return code, out.String(), errb.String()
}

func writeManifest(t *testing.T, dir, name string, m model.ToolManifest) {
	t.Helper()
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func manifest(name string, tier model.RiskTier, caps ...string) model.ToolManifest {
	return model.ToolManifest{
		Name: name, Version: "1.0.0", Description: "test " + name,
		InputSchema: map[string]any{"type": "object"},
		RiskTier:    tier, Capabilities: caps, TimeoutMS: 1000,
	}
}

func TestToolsKeygenSignVerifyList(t *testing.T) {
	keys := filepath.Join(t.TempDir(), "keys")
	code, out, errs := run(t, "tools", "keygen", "--out", keys)
	if code != ExitPass {
		t.Fatalf("keygen: %d %s", code, errs)
	}
	if !strings.Contains(out, "key id ") {
		t.Fatalf("keygen output %q", out)
	}
	st, err := os.Stat(filepath.Join(keys, PrivateKeyFile))
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("private key: %v %v", err, st)
	}
	// Never overwrite a key.
	if code, _, _ := run(t, "tools", "keygen", "--out", keys); code != ExitError {
		t.Fatal("second keygen should refuse")
	}

	manifests := t.TempDir()
	writeManifest(t, manifests, "http_get", manifest("http_get", model.TierLow, "http.read"))
	writeManifest(t, manifests, "report_finding", manifest("report_finding", model.TierLow, "report"))
	if err := os.WriteFile(filepath.Join(manifests, "notes.txt"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Unsigned manifests fail verification against any store.
	trust := filepath.Join(t.TempDir(), "trust")
	if err := os.MkdirAll(trust, 0o755); err != nil {
		t.Fatal(err)
	}
	pub, _ := os.ReadFile(filepath.Join(keys, PublicKeyFile))
	if err := os.WriteFile(filepath.Join(trust, "owner.pub"), pub, 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ = run(t, "tools", "verify", "--manifests", manifests, "--trust", trust)
	if code != ExitError || !strings.Contains(out, "UNSIGNED") {
		t.Fatalf("verify unsigned: %d %q", code, out)
	}

	code, out, errs = run(t, "tools", "sign", "--manifests", manifests, "--key", filepath.Join(keys, PrivateKeyFile))
	if code != ExitPass || !strings.Contains(out, "2 manifests") {
		t.Fatalf("sign: %d %q %q", code, out, errs)
	}
	code, out, errs = run(t, "tools", "verify", "--manifests", manifests, "--trust", trust)
	if code != ExitPass || strings.Count(out, "PROVENANCE_VALID") != 2 || !strings.Contains(out, "(owner)") {
		t.Fatalf("verify signed: %d %q %q", code, out, errs)
	}

	code, out, _ = run(t, "tools", "list", "--manifests", manifests)
	if code != ExitPass || !strings.Contains(out, "http_get") || strings.Contains(out, "unsigned") {
		t.Fatalf("list: %d %q", code, out)
	}

	// Tamper one manifest: verify fails with exit 2 and names the reason.
	p := filepath.Join(manifests, "http_get.json")
	raw, _ := os.ReadFile(p)
	raw = bytes.Replace(raw, []byte(`"risk_tier": "low"`), []byte(`"risk_tier": "critical"`), 1)
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, _ = run(t, "tools", "verify", "--manifests", manifests, "--trust", trust)
	if code != ExitError || !strings.Contains(out, "SIGNATURE_INVALID") || !strings.Contains(out, "PROVENANCE_VALID") {
		t.Fatalf("verify tampered: %d %q", code, out)
	}

	// Empty trust store fails closed.
	if code, _, _ := run(t, "tools", "verify", "--manifests", manifests, "--trust", t.TempDir()); code != ExitError {
		t.Fatal("empty trust store accepted")
	}

	// Usage errors.
	for _, args := range [][]string{
		{"tools"}, {"tools", "bogus"}, {"tools", "keygen"}, {"tools", "sign", "--manifests", manifests},
		{"tools", "verify", "--manifests", manifests}, {"tools", "list"}, {"tools", "list", "--manifests", t.TempDir()},
		{"tools", "sign", "--manifests", manifests, "--key", filepath.Join(keys, "missing")},
	} {
		if code, _, _ := run(t, args...); code != ExitError {
			t.Fatalf("%v should exit %d", args, ExitError)
		}
	}
	if code, _, _ := run(t, "tools", "help"); code != ExitPass {
		t.Fatal("tools help")
	}
}

func TestPolicyEval(t *testing.T) {
	manifests := t.TempDir()
	writeManifest(t, manifests, "http_get", manifest("http_get", model.TierLow, "http.read"))
	writeManifest(t, manifests, "http_post", manifest("http_post", model.TierHigh, "http.read", "http.write"))
	pol := filepath.Join("..", "..", "policy", "default.yaml")

	decide := func(args ...string) (int, model.Decision) {
		t.Helper()
		code, out, errs := run(t, append([]string{"policy", "eval", "--policy", pol, "--manifests", manifests}, args...)...)
		var d model.Decision
		if out != "" {
			if err := json.Unmarshal([]byte(out), &d); err != nil {
				t.Fatalf("not a decision: %q %q", out, errs)
			}
		}
		return code, d
	}

	// Without a trust store or --assume-signed the manifest is unverified.
	code, d := decide("--tool", "http_get", "--agent", "recon")
	if code != ExitBlocked || d.Reason != "SIGNATURE_UNVERIFIED" {
		t.Fatalf("%d %+v", code, d)
	}
	code, d = decide("--tool", "http_get", "--agent", "recon", "--assume-signed")
	if code != ExitPass || d.Effect != model.EffectAllow || d.Reason != "ALLOW_RULE:allow-read-only-tools" || d.PolicyHash == "" {
		t.Fatalf("%d %+v", code, d)
	}
	code, d = decide("--tool", "http_post", "--agent", "recon", "--assume-signed")
	if code != ExitBlocked || d.Reason != "TOOL_NOT_ALLOWED_FOR_AGENT" {
		t.Fatalf("%d %+v", code, d)
	}
	code, d = decide("--tool", "http_get", "--agent", "recon", "--assume-signed", "--depth", "2", "--ancestors", "orchestrator,probe")
	if code != ExitBlocked || d.Reason != "DEPTH_EXCEEDED" {
		t.Fatalf("%d %+v", code, d)
	}
	code, d = decide("--tool", "http_get", "--agent", "recon", "--assume-signed", "--spent", "99")
	if code != ExitBlocked || d.Reason != "BUDGET_EXCEEDED" {
		t.Fatalf("%d %+v", code, d)
	}

	// With a real trust store the unsigned manifest is denied as UNSIGNED.
	keys := filepath.Join(t.TempDir(), "k")
	if code, _, _ := run(t, "tools", "keygen", "--out", keys); code != ExitPass {
		t.Fatal("keygen")
	}
	trust := filepath.Join(t.TempDir(), "trust")
	if err := os.MkdirAll(trust, 0o755); err != nil {
		t.Fatal(err)
	}
	pub, _ := os.ReadFile(filepath.Join(keys, PublicKeyFile))
	if err := os.WriteFile(filepath.Join(trust, "ci.pub"), pub, 0o644); err != nil {
		t.Fatal(err)
	}
	code, d = decide("--tool", "http_get", "--agent", "recon", "--trust", trust)
	if code != ExitBlocked || d.Reason != "UNSIGNED" {
		t.Fatalf("%d %+v", code, d)
	}
	if code, _, _ := run(t, "tools", "sign", "--manifests", manifests, "--key", filepath.Join(keys, PrivateKeyFile)); code != ExitPass {
		t.Fatal("sign")
	}
	code, d = decide("--tool", "http_get", "--agent", "recon", "--trust", trust)
	if code != ExitPass || d.Effect != model.EffectAllow {
		t.Fatalf("%d %+v", code, d)
	}

	// Errors.
	for _, args := range [][]string{
		{"policy"}, {"policy", "nope"},
		{"policy", "eval", "--policy", pol, "--manifests", manifests, "--agent", "recon"},
		{"policy", "eval", "--policy", pol, "--manifests", manifests, "--agent", "recon", "--tool", "missing.tool", "--assume-signed"},
		{"policy", "eval", "--policy", filepath.Join(t.TempDir(), "no.yaml"), "--manifests", manifests, "--agent", "recon", "--tool", "http_get", "--assume-signed"},
		{"policy", "eval", "--policy", pol, "--manifests", manifests, "--agent", "recon", "--tool", "http_get", "--trust", t.TempDir()},
	} {
		if code, _, _ := run(t, args...); code != ExitError {
			t.Fatalf("%v should exit %d", args, ExitError)
		}
	}
}

func TestPolicyLint(t *testing.T) {
	manifests := t.TempDir()
	writeManifest(t, manifests, "http_get", manifest("http_get", model.TierLow, "http.read"))
	writeManifest(t, manifests, "http_post", manifest("http_post", model.TierHigh, "http.read", "http.write"))
	pol := filepath.Join("..", "..", "policy", "default.yaml")

	// Clean lint against default policy.
	code, stdout, stderr := run(t, "policy", "lint", "--policy", pol, "--manifests", manifests)
	// The default policy references tools not in our test manifests (e.g. delegate,
	// report_finding), so it should flag issues.
	if code != ExitBlocked {
		t.Fatalf("default policy lint: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	var issues []map[string]any
	if err := json.Unmarshal([]byte(stdout), &issues); err != nil {
		t.Fatalf("lint output not JSON: %v stdout=%q", err, stdout)
	}
	if len(issues) == 0 {
		t.Fatal("expected issues for default policy with partial manifests")
	}

	// Build a tight policy that matches the manifests exactly.
	tight := filepath.Join(t.TempDir(), "tight.yaml")
	tightYAML := `
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: allow-probe
    effect: allow
    match:
      agents: [probe]
      tools: [http_get, http_post]
`
	if err := os.WriteFile(tight, []byte(tightYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = run(t, "policy", "lint", "--policy", tight, "--manifests", manifests)
	if code != ExitPass {
		t.Fatalf("tight policy should lint clean: code=%d stdout=%q", code, stdout)
	}

	// A policy with a ghost agent should produce an error-severity issue.
	ghostPol := filepath.Join(t.TempDir(), "ghost.yaml")
	ghostYAML := `
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: r1
    effect: deny
    match:
      agents: [nonexistent_agent]
`
	if err := os.WriteFile(ghostPol, []byte(ghostYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = run(t, "policy", "lint", "--policy", ghostPol, "--manifests", manifests)
	if code != ExitBlocked {
		t.Fatalf("ghost agent should cause issues: code=%d", code)
	}
	if !strings.Contains(stdout, "unknown_agent_in_rule") {
		t.Fatalf("expected unknown_agent_in_rule in %q", stdout)
	}

	// Error cases.
	for _, args := range [][]string{
		{"policy", "lint"},
		{"policy", "lint", "--policy", tight},
		{"policy", "lint", "--manifests", manifests},
		{"policy", "lint", "--policy", filepath.Join(t.TempDir(), "nope.yaml"), "--manifests", manifests},
	} {
		if code, _, _ := run(t, args...); code != ExitError {
			t.Fatalf("%v should exit %d, got %d", args, ExitError, code)
		}
	}
}

func TestPolicyTest(t *testing.T) {
	manifests := t.TempDir()
	writeManifest(t, manifests, "http_get", manifest("http_get", model.TierLow, "http.read"))
	writeManifest(t, manifests, "http_post", manifest("http_post", model.TierHigh, "http.read", "http.write"))

	pol := filepath.Join(t.TempDir(), "pol.yaml")
	polYAML := `
version: "1"
default_effect: deny
agents:
  recon:
    max_tier: medium
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget: {max_usd: 2}
rules:
  - id: allow-read
    effect: allow
    match:
      capabilities: [http.read]
      max_tier: medium
  - id: allow-probe-write
    effect: allow
    match:
      agents: [probe]
      tools: [http_post]
`
	if err := os.WriteFile(pol, []byte(polYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	// All-pass scenarios.
	passFile := filepath.Join(t.TempDir(), "pass.json")
	passJSON := `[
  {"agent":"recon","tool":"http_get","expect":"allow","label":"recon reads"},
  {"agent":"recon","tool":"http_post","expect":"deny","label":"recon cannot write"},
  {"agent":"probe","tool":"http_post","expect":"allow","label":"probe writes"}
]`
	if err := os.WriteFile(passFile, []byte(passJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "policy", "test", "--policy", pol, "--manifests", manifests, "--scenarios", passFile)
	if code != ExitPass {
		t.Fatalf("all-pass: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	var results []map[string]any
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatalf("output not JSON: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if r["pass"] != true {
			t.Fatalf("result %d should pass: %v", i, r)
		}
	}

	// One failing scenario.
	failFile := filepath.Join(t.TempDir(), "fail.json")
	failJSON := `[
  {"agent":"recon","tool":"http_get","expect":"deny","label":"wrong expectation"}
]`
	if err := os.WriteFile(failFile, []byte(failJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = run(t, "policy", "test", "--policy", pol, "--manifests", manifests, "--scenarios", failFile)
	if code != ExitBlocked {
		t.Fatalf("should fail: code=%d", code)
	}
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatal(err)
	}
	if results[0]["pass"] != false {
		t.Fatalf("expected fail: %v", results[0])
	}

	// Error cases.
	for _, args := range [][]string{
		{"policy", "test"},
		{"policy", "test", "--policy", pol, "--manifests", manifests},
		{"policy", "test", "--policy", pol, "--scenarios", passFile},
		{"policy", "test", "--manifests", manifests, "--scenarios", passFile},
		{"policy", "test", "--policy", filepath.Join(t.TempDir(), "nope.yaml"), "--manifests", manifests, "--scenarios", passFile},
	} {
		if code, _, _ := run(t, args...); code != ExitError {
			t.Fatalf("%v should exit %d, got %d", args, ExitError, code)
		}
	}
}

func TestPolicyCoverage(t *testing.T) {
	manifests := t.TempDir()
	writeManifest(t, manifests, "http_get", manifest("http_get", model.TierLow, "http.read"))
	writeManifest(t, manifests, "http_post", manifest("http_post", model.TierHigh, "http.read", "http.write"))
	writeManifest(t, manifests, "score_task", manifest("score_task", model.TierLow, "report"))

	// Full coverage: every agent×tool pair has an explicit rule.
	fullPol := filepath.Join(t.TempDir(), "full.yaml")
	fullYAML := `
version: "1"
default_effect: deny
agents:
  recon:
    max_tier: medium
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
  scorer:
    max_tier: low
    allowed_tools: [score_task]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: allow-read
    effect: allow
    match:
      capabilities: [http.read]
      max_tier: medium
  - id: allow-score
    effect: allow
    match:
      agents: [scorer]
      tools: [score_task]
`
	if err := os.WriteFile(fullPol, []byte(fullYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "policy", "coverage", "--policy", fullPol, "--manifests", manifests)
	if code != ExitPass {
		t.Fatalf("full coverage: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if report["coverage_pct"] != 100.0 {
		t.Fatalf("expected 100%%, got %v", report["coverage_pct"])
	}

	// Partial coverage: agent has a tool with no matching rule.
	partialPol := filepath.Join(t.TempDir(), "partial.yaml")
	partialYAML := `
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget: {max_usd: 2}
rules:
  - id: allow-read
    effect: allow
    match:
      capabilities: [http.read]
      max_tier: medium
`
	if err := os.WriteFile(partialPol, []byte(partialYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ = run(t, "policy", "coverage", "--policy", partialPol, "--manifests", manifests)
	if code != ExitBlocked {
		t.Fatalf("partial coverage should exit blocked: code=%d", code)
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	uncovered, ok := report["uncovered"].([]any)
	if !ok || len(uncovered) == 0 {
		t.Fatal("expected uncovered pairs")
	}
	first := uncovered[0].(map[string]any)
	if first["tool"] != "http_post" {
		t.Fatalf("expected http_post uncovered, got %v", first["tool"])
	}

	// Error cases.
	for _, args := range [][]string{
		{"policy", "coverage"},
		{"policy", "coverage", "--policy", fullPol},
		{"policy", "coverage", "--manifests", manifests},
		{"policy", "coverage", "--policy", filepath.Join(t.TempDir(), "nope.yaml"), "--manifests", manifests},
	} {
		if code, _, _ := run(t, args...); code != ExitError {
			t.Fatalf("%v should exit %d, got %d", args, ExitError, code)
		}
	}
}

func TestAuditCommands(t *testing.T) {
	code, out, _ := run(t, "audit", "keygen")
	if code != ExitPass {
		t.Fatal("keygen")
	}
	keyText := strings.TrimSpace(out)
	master, err := audit.ParseMaster(keyText)
	if err != nil {
		t.Fatalf("keygen output not a key: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "audit.log")
	w, err := audit.OpenWriter(logPath, "run-cli", master)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, err := w.Record(context.Background(), model.AuditRecord{Kind: "tool_call", Tool: "http_get", Meta: map[string]any{"i": i}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	code, out, errs := run(t, "audit", "verify", "--log", logPath)
	if code != ExitPass || !strings.Contains(out, `"records": 3`) || strings.Contains(out, `"by_kind"`) {
		t.Fatalf("keyless verify: %d %q %q", code, out, errs)
	}

	t.Setenv("ASSAY_AUDIT_KEY", keyText)
	code, out, errs = run(t, "audit", "verify", "--log", logPath, "--key-env", "ASSAY_AUDIT_KEY")
	if code != ExitPass || !strings.Contains(out, `"tool_call": 3`) {
		t.Fatalf("keyed verify: %d %q %q", code, out, errs)
	}

	code, out, errs = run(t, "audit", "export", "--log", logPath, "--key-env", "ASSAY_AUDIT_KEY")
	if code != ExitPass || strings.Count(out, "\n") != 3 || !strings.Contains(errs, "exported 3 records") {
		t.Fatalf("export: %d %q %q", code, out, errs)
	}

	t.Setenv("ASSAY_OTHER_KEY", audit.EncodeMaster(audit.GenerateMaster()))
	if code, _, errs := run(t, "audit", "verify", "--log", logPath, "--key-env", "ASSAY_OTHER_KEY"); code != ExitError || !strings.Contains(errs, "wrong key") {
		t.Fatalf("wrong key: %d %q", code, errs)
	}

	// Corrupt the log: verify fails with the line number.
	raw, _ := os.ReadFile(logPath)
	if err := os.WriteFile(logPath, raw[:len(raw)-5], 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, errs := run(t, "audit", "verify", "--log", logPath); code != ExitError || !strings.Contains(errs, "line 4") {
		t.Fatalf("truncated: %d %q", code, errs)
	}

	for _, args := range [][]string{
		{"audit"}, {"audit", "nope"}, {"audit", "verify"}, {"audit", "export", "--log", logPath},
		{"audit", "verify", "--log", logPath, "--key-env", "ASSAY_UNSET_KEY"},
		{"audit", "export", "--log", logPath, "--key-env", "ASSAY_UNSET_KEY"},
		{"audit", "keygen", "extra"},
	} {
		if code, _, _ := run(t, args...); code != ExitError {
			t.Fatalf("%v should exit %d", args, ExitError)
		}
	}
	if code, _, _ := run(t, "audit", "help"); code != ExitPass {
		t.Fatal("audit help")
	}
}

func TestAuditVerifyCrossCheck(t *testing.T) {
	master := audit.GenerateMaster()
	logPath := filepath.Join(t.TempDir(), "audit.log")
	w, err := audit.OpenWriter(logPath, "run-xc", master)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := w.Record(context.Background(), model.AuditRecord{Kind: "tool_call", Tool: "http_get"}); err != nil {
			t.Fatal(err)
		}
	}
	_, headHash := w.Head()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	runDir := filepath.Join(t.TempDir(), "results", "run-xc")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	rr := model.RunReport{
		RunID:      "run-xc",
		AuditHead:  headHash,
		AuditCount: 5,
		Verdict:    "PASS",
		ExitCode:   0,
	}
	b, _ := json.Marshal(rr)
	if err := os.WriteFile(filepath.Join(runDir, "run.json"), b, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ASSAY_XC_KEY", audit.EncodeMaster(master))
	code, out, errs := run(t, "audit", "verify", "--log", logPath, "--key-env", "ASSAY_XC_KEY", "--run", runDir)
	if code != ExitPass {
		t.Fatalf("cross-check should pass: code=%d stderr=%q", code, errs)
	}
	if !strings.Contains(out, `"cross_checked": true`) {
		t.Errorf("output should contain cross_checked: true, got %q", out)
	}

	// Mismatch: wrong record count in run.json.
	rr.AuditCount = 999
	b, _ = json.Marshal(rr)
	os.WriteFile(filepath.Join(runDir, "run.json"), b, 0o600)
	code, _, errs = run(t, "audit", "verify", "--log", logPath, "--key-env", "ASSAY_XC_KEY", "--run", runDir)
	if code != ExitBlocked || !strings.Contains(errs, "MISMATCH") {
		t.Fatalf("count mismatch: code=%d stderr=%q", code, errs)
	}

	// Mismatch: wrong head hash in run.json.
	rr.AuditCount = 5
	rr.AuditHead = "0000000000000000000000000000000000000000000000000000000000000000"
	b, _ = json.Marshal(rr)
	os.WriteFile(filepath.Join(runDir, "run.json"), b, 0o600)
	code, _, errs = run(t, "audit", "verify", "--log", logPath, "--key-env", "ASSAY_XC_KEY", "--run", runDir)
	if code != ExitBlocked || !strings.Contains(errs, "MISMATCH") || !strings.Contains(errs, "head_hash") {
		t.Fatalf("hash mismatch: code=%d stderr=%q", code, errs)
	}
}

func TestPolicySimulate(t *testing.T) {
	manifests := t.TempDir()
	writeManifest(t, manifests, "http_get", manifest("http_get", model.TierLow, "http.read"))
	writeManifest(t, manifests, "http_post", manifest("http_post", model.TierHigh, "http.read", "http.write"))

	pol := filepath.Join(t.TempDir(), "policy.yaml")
	yaml := `
version: "1"
default_effect: deny
agents:
  recon:
    max_tier: low
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: allow-read
    effect: allow
    match:
      capabilities: [http.read]
      max_tier: low
  - id: allow-probe-write
    effect: allow
    match:
      agents: [probe]
      tools: [http_post]
`
	if err := os.WriteFile(pol, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := run(t, "policy", "simulate", "--policy", pol, "--manifests", manifests)
	if code != ExitPass {
		t.Fatalf("simulate: code=%d stderr=%q", code, stderr)
	}

	var report simReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("not JSON: %v\nstdout: %s", err, stdout)
	}
	if len(report.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(report.Agents))
	}
	if len(report.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(report.Tools))
	}
	// 2 agents × 2 tools = 4 cells
	if len(report.Cells) != 4 {
		t.Fatalf("expected 4 cells, got %d", len(report.Cells))
	}
	total := report.Allowed + report.Denied
	if total != 4 {
		t.Fatalf("expected 4 total, got %d", total)
	}

	// recon+http_get = allow (allow-read matches), recon+http_post = deny (tier exceeds)
	// probe+http_get = allow (allow-read matches), probe+http_post = allow (allow-probe-write)
	if report.Allowed != 3 || report.Denied != 1 {
		t.Fatalf("expected 3 allowed 1 denied, got %d allowed %d denied", report.Allowed, report.Denied)
	}

	// Check the denied cell
	for _, c := range report.Cells {
		if c.Agent == "recon" && c.Tool == "http_post" {
			if c.Effect != "deny" {
				t.Fatalf("recon+http_post should be deny, got %s (%s)", c.Effect, c.Reason)
			}
		}
	}
}

func TestPolicySimulateFiltered(t *testing.T) {
	manifests := t.TempDir()
	writeManifest(t, manifests, "http_get", manifest("http_get", model.TierLow, "http.read"))
	writeManifest(t, manifests, "http_post", manifest("http_post", model.TierHigh, "http.write"))

	pol := filepath.Join(t.TempDir(), "policy.yaml")
	yaml := `
version: "1"
default_effect: allow
agents:
  recon:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget: {max_usd: 1}
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules: []
`
	if err := os.WriteFile(pol, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}

	// Filter to one agent
	code, stdout, _ := run(t, "policy", "simulate", "--policy", pol, "--manifests", manifests, "--agent", "recon")
	if code != ExitPass {
		t.Fatalf("filtered simulate: code=%d", code)
	}
	var report simReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Agents) != 1 || report.Agents[0] != "recon" {
		t.Fatalf("expected [recon], got %v", report.Agents)
	}
	if len(report.Cells) != 2 {
		t.Fatalf("expected 2 cells (1 agent × 2 tools), got %d", len(report.Cells))
	}

	// Filter to one tool
	code, stdout, _ = run(t, "policy", "simulate", "--policy", pol, "--manifests", manifests, "--tool", "http_get")
	if code != ExitPass {
		t.Fatalf("tool-filtered simulate: code=%d", code)
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Tools) != 1 || report.Tools[0] != "http_get" {
		t.Fatalf("expected [http_get], got %v", report.Tools)
	}
	if len(report.Cells) != 2 {
		t.Fatalf("expected 2 cells (2 agents × 1 tool), got %d", len(report.Cells))
	}
}

func TestPolicyDiff(t *testing.T) {
	beforePol := filepath.Join(t.TempDir(), "before.yaml")
	beforeYAML := `
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: allow-get
    effect: allow
    match:
      tools: [http_get]
  - id: deny-post
    effect: deny
    match:
      tools: [http_post]
`
	if err := os.WriteFile(beforePol, []byte(beforeYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	afterPol := filepath.Join(t.TempDir(), "after.yaml")
	afterYAML := `
version: "2"
default_effect: allow
agents:
  probe:
    max_tier: critical
    allowed_tools: [http_get]
    max_delegation_depth: 2
    budget: {max_usd: 5}
rules:
  - id: allow-get
    effect: allow
    match:
      tools: [http_get]
  - id: deny-all
    effect: deny
    match:
      agents: [probe]
`
	if err := os.WriteFile(afterPol, []byte(afterYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	// Identical policies should exit pass.
	code, stdout, stderr := run(t, "policy", "diff", "--before", beforePol, "--after", beforePol)
	if code != ExitPass {
		t.Fatalf("identical diff: code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "identical") {
		t.Fatalf("expected 'identical' in output, got %q", stdout)
	}

	// Different policies should exit blocked with text output.
	code, stdout, stderr = run(t, "policy", "diff", "--before", beforePol, "--after", afterPol)
	if code != ExitBlocked {
		t.Fatalf("diff with changes: code=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{"version:", "default_effect:", "Rules added:", "Rules removed:", "Agents changed:"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %q in text output, got %q", want, stdout)
		}
	}

	// JSON format.
	code, stdout, _ = run(t, "policy", "diff", "--before", beforePol, "--after", afterPol, "--format", "json")
	if code != ExitBlocked {
		t.Fatalf("json diff: code=%d", code)
	}
	var report map[string]any
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("not JSON: %v\nstdout: %s", err, stdout)
	}
	if report["identical"] != false {
		t.Fatal("expected identical=false")
	}
	if report["version_changed"] == nil {
		t.Fatal("expected version_changed in JSON")
	}

	// Error cases.
	for _, args := range [][]string{
		{"policy", "diff"},
		{"policy", "diff", "--before", beforePol},
		{"policy", "diff", "--after", afterPol},
		{"policy", "diff", "--before", filepath.Join(t.TempDir(), "nope.yaml"), "--after", afterPol},
		{"policy", "diff", "--before", beforePol, "--after", filepath.Join(t.TempDir(), "nope.yaml")},
	} {
		if code, _, _ := run(t, args...); code != ExitError {
			t.Fatalf("%v should exit %d, got %d", args, ExitError, code)
		}
	}
}

func TestPolicySimulateErrors(t *testing.T) {
	// Missing required flags
	code, _, stderr := run(t, "policy", "simulate")
	if code != ExitError || !strings.Contains(stderr, "required") {
		t.Fatalf("missing flags: code=%d stderr=%q", code, stderr)
	}

	// Nonexistent agent filter
	manifests := t.TempDir()
	writeManifest(t, manifests, "http_get", manifest("http_get", model.TierLow))
	pol := filepath.Join(t.TempDir(), "policy.yaml")
	yaml := `
version: "1"
default_effect: deny
agents:
  recon:
    max_tier: low
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules: []
`
	if err := os.WriteFile(pol, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = run(t, "policy", "simulate", "--policy", pol, "--manifests", manifests, "--agent", "nonexistent")
	if code != ExitError || !strings.Contains(stderr, "no agents") {
		t.Fatalf("bad agent filter: code=%d stderr=%q", code, stderr)
	}
}

func TestPolicyMerge(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.yaml")
	baseYAML := `
version: "1"
default_effect: deny
agents:
  recon:
    max_tier: low
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: allow-get
    effect: allow
    match:
      tools: [http_get]
`
	if err := os.WriteFile(base, []byte(baseYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	overlay := filepath.Join(dir, "overlay.yaml")
	overlayYAML := `
version: "2"
default_effect: allow
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 1
    budget: {max_usd: 5}
rules:
  - id: allow-get
    effect: allow
    match:
      agents: [probe]
  - id: deny-post
    effect: deny
    match:
      tools: [http_post]
`
	if err := os.WriteFile(overlay, []byte(overlayYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	// JSON output to stdout.
	code, stdout, stderr := run(t, "policy", "merge", "--policies", base+","+overlay, "--format", "json")
	if code != ExitPass {
		t.Fatalf("merge json: code=%d stderr=%q", code, stderr)
	}
	var merged map[string]any
	if err := json.Unmarshal([]byte(stdout), &merged); err != nil {
		t.Fatalf("not JSON: %v\nstdout: %s", err, stdout)
	}
	if merged["version"] != "2" {
		t.Fatalf("expected version 2, got %v", merged["version"])
	}
	if merged["default_effect"] != "allow" {
		t.Fatalf("expected default_effect allow, got %v", merged["default_effect"])
	}
	agents := merged["agents"].(map[string]any)
	if len(agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(agents))
	}
	rules := merged["rules"].([]any)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules (allow-get overridden + deny-post), got %d", len(rules))
	}

	// YAML output to file.
	outFile := filepath.Join(dir, "merged.yaml")
	code, _, stderr = run(t, "policy", "merge", "--policies", base+","+overlay, "--format", "yaml", "--out", outFile)
	if code != ExitPass {
		t.Fatalf("merge yaml: code=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(outFile); err != nil {
		t.Fatalf("expected output file: %v", err)
	}

	// Error cases.
	for _, args := range [][]string{
		{"policy", "merge"},
		{"policy", "merge", "--policies", filepath.Join(t.TempDir(), "nope.yaml")},
	} {
		if code, _, _ := run(t, args...); code != ExitError {
			t.Fatalf("%v should exit %d, got %d", args, ExitError, code)
		}
	}
}

func TestPolicyValidate(t *testing.T) {
	validPol := filepath.Join(t.TempDir(), "valid.yaml")
	validYAML := `
version: "1"
default_effect: deny
agents:
  probe:
    max_tier: high
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: allow-get
    effect: allow
    match:
      tools: [http_get]
`
	if err := os.WriteFile(validPol, []byte(validYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "policy", "validate", "--policy", validPol)
	if code != ExitPass {
		t.Fatalf("valid policy: code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stderr, "valid") {
		t.Fatalf("expected 'valid' in stderr, got %q", stderr)
	}
	var issues []map[string]any
	if err := json.Unmarshal([]byte(stdout), &issues); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}

	badPol := filepath.Join(t.TempDir(), "bad.yaml")
	badYAML := `
version: ""
default_effect: maybe
agents:
  probe:
    max_tier: ultra
    allowed_tools: [http_get]
    max_delegation_depth: 0
    budget: {max_usd: 1}
rules:
  - id: r1
    effect: allow
    match:
      tools: [http_get]
  - id: r1
    effect: permit
    match:
      tools: [http_get]
dangerous_combinations:
  - [only_one]
`
	if err := os.WriteFile(badPol, []byte(badYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = run(t, "policy", "validate", "--policy", badPol)
	if code != ExitBlocked {
		t.Fatalf("bad policy: code=%d stderr=%q", code, stderr)
	}
	if err := json.Unmarshal([]byte(stdout), &issues); err != nil {
		t.Fatalf("not JSON: %v\nstdout: %s", err, stdout)
	}
	if len(issues) < 4 {
		t.Fatalf("expected at least 4 issues, got %d", len(issues))
	}

	for _, args := range [][]string{
		{"policy", "validate"},
		{"policy", "validate", "--policy", filepath.Join(t.TempDir(), "nope.yaml")},
	} {
		if code, _, _ := run(t, args...); code != ExitError {
			t.Fatalf("validate %v should exit %d, got %d", args, ExitError, code)
		}
	}
}

func TestPolicyExport(t *testing.T) {
	polYAML := `
version: "2.1"
default_effect: deny
agents:
  recon:
    max_tier: medium
    allowed_tools: [http_get]
    max_delegation_depth: 1
    budget: {max_usd: 0.5}
  probe:
    max_tier: high
    allowed_tools: [http_get, http_post]
    max_delegation_depth: 0
    budget: {max_usd: 2}
rules:
  - id: allow-read
    effect: allow
    match:
      tools: [http_get]
    rationale: "Read access for discovery"
dangerous_combinations:
  - [http_post, exec_shell]
`
	polPath := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(polPath, []byte(polYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := run(t, "policy", "export", "--policy", polPath)
	if code != ExitPass {
		t.Fatalf("markdown export: code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "# Policy: 2.1") {
		t.Error("markdown should contain policy version heading")
	}
	if !strings.Contains(stdout, "| probe |") {
		t.Error("markdown should contain agent table rows")
	}
	if !strings.Contains(stdout, "### `allow-read` (allow)") {
		t.Error("markdown should contain rule heading")
	}
	if !strings.Contains(stdout, "http_post + exec_shell") {
		t.Error("markdown should contain dangerous combos")
	}

	code, stdout, _ = run(t, "policy", "export", "--policy", polPath, "--format", "json")
	if code != ExitPass {
		t.Fatalf("json export failed: code=%d", code)
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(stdout), &p); err != nil {
		t.Fatalf("json export not valid JSON: %v", err)
	}
	if p["version"] != "2.1" {
		t.Errorf("json version = %v, want 2.1", p["version"])
	}

	code, stdout, _ = run(t, "policy", "export", "--policy", polPath, "--format", "yaml")
	if code != ExitPass {
		t.Fatalf("yaml export failed: code=%d", code)
	}
	if !strings.Contains(stdout, "default_effect: deny") {
		t.Error("yaml should contain default_effect")
	}

	for _, args := range [][]string{
		{"policy", "export"},
		{"policy", "export", "--policy", filepath.Join(t.TempDir(), "nope.yaml")},
		{"policy", "export", "--policy", polPath, "--format", "xml"},
	} {
		if code, _, _ := run(t, args...); code != ExitError {
			t.Fatalf("export %v should exit %d, got %d", args, ExitError, code)
		}
	}
}

func TestPolicyTemplate(t *testing.T) {
	for _, preset := range []string{"minimal", "standard", "strict"} {
		t.Run(preset, func(t *testing.T) {
			code, stdout, _ := run(t, "policy", "template", "--preset", preset)
			if code != ExitPass {
				t.Fatalf("template %s: code=%d", preset, code)
			}
			if !strings.Contains(stdout, "version:") {
				t.Error("template output should contain version")
			}
			if !strings.Contains(stdout, "agents:") {
				t.Error("template output should contain agents")
			}
		})
	}

	outDir := t.TempDir()
	outFile := filepath.Join(outDir, "policy.yaml")
	code, _, _ := run(t, "policy", "template", "--preset", "standard", "--out", outFile)
	if code != ExitPass {
		t.Fatalf("template --out: code=%d", code)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if !strings.Contains(string(data), "agents:") {
		t.Error("written file should contain agents")
	}

	code, _, _ = run(t, "policy", "template")
	if code != ExitError {
		t.Fatal("template with no --preset should error")
	}
	code, _, _ = run(t, "policy", "template", "--preset", "nonexistent")
	if code != ExitError {
		t.Fatal("template with bad preset should error")
	}
}
