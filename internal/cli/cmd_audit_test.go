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

func writeAuditLog(t *testing.T, dir string, n int) (logPath string, master []byte, headHash string, count uint64) {
	t.Helper()
	logPath = filepath.Join(dir, "audit.log")
	master = audit.GenerateMaster()
	w, err := audit.OpenWriter(logPath, "run-test-001", master)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		if _, err := w.Record(context.Background(), model.AuditRecord{
			Kind: "tool_call",
			Tool: "http.get",
		}); err != nil {
			t.Fatal(err)
		}
	}
	count, headHash = w.Head()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return
}

func writeRunJSON(t *testing.T, dir, headHash string, count uint64) {
	t.Helper()
	rr := model.RunReport{
		RunID:      "run-test-001",
		Verdict:    "PASS",
		ExitCode:   0,
		AuditHead:  headHash,
		AuditCount: count,
	}
	b, _ := json.MarshalIndent(rr, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "run.json"), b, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestAuditVerifyBasic(t *testing.T) {
	dir := t.TempDir()
	logPath, master, _, _ := writeAuditLog(t, dir, 3)

	t.Setenv("TEST_AUDIT_KEY", audit.EncodeMaster(master))

	var stdout, stderr bytes.Buffer
	code := runAudit([]string{"verify", "--log", logPath, "--key-env", "TEST_AUDIT_KEY"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"records": 3`) {
		t.Errorf("expected 3 records in output; got: %s", stdout.String())
	}
}

func TestAuditVerifyWithRunMatch(t *testing.T) {
	dir := t.TempDir()
	logPath, master, headHash, count := writeAuditLog(t, dir, 5)
	writeRunJSON(t, dir, headHash, count)

	t.Setenv("TEST_AUDIT_KEY", audit.EncodeMaster(master))

	var stdout, stderr bytes.Buffer
	code := runAudit([]string{"verify", "--log", logPath, "--key-env", "TEST_AUDIT_KEY", "--run", dir}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"cross_checked": true`) {
		t.Error("expected cross_checked: true in output")
	}
}

func TestAuditVerifyWithRunMismatch(t *testing.T) {
	dir := t.TempDir()
	logPath, master, _, count := writeAuditLog(t, dir, 5)
	writeRunJSON(t, dir, "0000000000000000000000000000000000000000000000000000000000000000", count)

	t.Setenv("TEST_AUDIT_KEY", audit.EncodeMaster(master))

	var stdout, stderr bytes.Buffer
	code := runAudit([]string{"verify", "--log", logPath, "--key-env", "TEST_AUDIT_KEY", "--run", dir}, &stdout, &stderr)
	if code != ExitBlocked {
		t.Fatalf("expected ExitBlocked, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "MISMATCH") {
		t.Error("expected MISMATCH in stderr")
	}
}

func TestAuditVerifyWithRunCountMismatch(t *testing.T) {
	dir := t.TempDir()
	logPath, master, headHash, _ := writeAuditLog(t, dir, 5)
	writeRunJSON(t, dir, headHash, 999)

	t.Setenv("TEST_AUDIT_KEY", audit.EncodeMaster(master))

	var stdout, stderr bytes.Buffer
	code := runAudit([]string{"verify", "--log", logPath, "--key-env", "TEST_AUDIT_KEY", "--run", dir}, &stdout, &stderr)
	if code != ExitBlocked {
		t.Fatalf("expected ExitBlocked, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "record_count") {
		t.Error("expected record_count mismatch in stderr")
	}
}

func TestAuditKeygen(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runAudit([]string{"keygen"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	key := strings.TrimSpace(stdout.String())
	if len(key) < 40 {
		t.Errorf("key looks too short: %q", key)
	}
}

func TestAuditNoArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runAudit(nil, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}

func TestAuditVerifyNoLog(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runAudit([]string{"verify"}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}
