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
