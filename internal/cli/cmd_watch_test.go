package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSnapshotFiles(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	os.WriteFile(f1, []byte("hello"), 0o644)
	f2 := filepath.Join(dir, "b.txt")

	snap := snapshotFiles([]string{f1, f2})
	if snap[f1] <= 0 {
		t.Errorf("expected positive mod time for existing file, got %d", snap[f1])
	}
	if snap[f2] != -1 {
		t.Errorf("expected -1 for missing file, got %d", snap[f2])
	}
}

func TestFilesChangedDetectsChange(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("v1"), 0o644)
	before := snapshotFiles([]string{f})

	time.Sleep(10 * time.Millisecond)
	os.WriteFile(f, []byte("v2"), 0o644)
	// Force a different mod time
	now := time.Now().Add(time.Second)
	os.Chtimes(f, now, now)
	after := snapshotFiles([]string{f})

	if !filesChanged(before, after) {
		t.Error("expected filesChanged to detect modification")
	}
}

func TestFilesChangedNoChange(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "a.txt")
	os.WriteFile(f, []byte("v1"), 0o644)
	snap := snapshotFiles([]string{f})

	if filesChanged(snap, snap) {
		t.Error("expected no change for identical snapshots")
	}
}

func TestFilesChangedNewFile(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	os.WriteFile(f1, []byte("v1"), 0o644)
	before := snapshotFiles([]string{f1})

	f2 := filepath.Join(dir, "b.txt")
	os.WriteFile(f2, []byte("v2"), 0o644)
	after := snapshotFiles([]string{f1, f2})

	if !filesChanged(before, after) {
		t.Error("expected filesChanged to detect new file")
	}
}

func TestFilesChangedRemovedFile(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	os.WriteFile(f1, []byte("v1"), 0o644)
	os.WriteFile(f2, []byte("v2"), 0o644)
	before := snapshotFiles([]string{f1, f2})
	after := snapshotFiles([]string{f1})

	if !filesChanged(before, after) {
		t.Error("expected filesChanged to detect removed file")
	}
}

func TestWatchMissingFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWatch([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
	if !strings.Contains(stderr.String(), "required") {
		t.Errorf("expected 'required' in stderr, got: %s", stderr.String())
	}
}

func TestWatchMaxCycles(t *testing.T) {
	dir := t.TempDir()
	policyFile := filepath.Join(dir, "policy.yaml")
	manifestDir := filepath.Join(dir, "manifests")
	os.MkdirAll(manifestDir, 0o755)

	// Write a minimal valid policy
	os.WriteFile(policyFile, []byte(`default_effect: deny
agents:
  tester:
    allowed_tools: [example]
    budget: {max_usd: 1, max_latency_ms: 1000}
rules:
  - effect: allow
    match: {agents: [tester], tools: [example]}
    rationale: test
`), 0o644)

	// Write a manifest
	os.WriteFile(filepath.Join(manifestDir, "example.json"),
		[]byte(`{"name":"example","version":"1.0","risk_tier":"low","capabilities":["http.read"]}`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runWatch([]string{
		"--policy", policyFile,
		"--manifests", manifestDir,
		"--interval", "0.01",
		"--max-cycles", "2",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "watching") {
		t.Errorf("expected 'watching' in output, got: %s", stdout.String())
	}
}

func TestWatchJSONFormat(t *testing.T) {
	dir := t.TempDir()
	policyFile := filepath.Join(dir, "policy.yaml")
	manifestDir := filepath.Join(dir, "manifests")
	os.MkdirAll(manifestDir, 0o755)

	os.WriteFile(policyFile, []byte(`default_effect: deny
agents:
  tester:
    allowed_tools: [example]
    budget: {max_usd: 1, max_latency_ms: 1000}
rules:
  - effect: allow
    match: {agents: [tester], tools: [example]}
    rationale: test
`), 0o644)
	os.WriteFile(filepath.Join(manifestDir, "example.json"),
		[]byte(`{"name":"example","version":"1.0","risk_tier":"low","capabilities":["http.read"]}`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runWatch([]string{
		"--policy", policyFile,
		"--manifests", manifestDir,
		"--interval", "0.01",
		"--max-cycles", "1",
		"--format", "json",
	}, &stdout, &stderr)
	if code != ExitPass {
		t.Errorf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"event"`) {
		t.Errorf("expected JSON with 'event' key, got: %s", stdout.String())
	}
}
