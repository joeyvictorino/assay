package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFindingsDedupBasic(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "findings.json")
	os.WriteFile(input, []byte(`[
		{"checker":"sqli","class":"injection","severity":"high","message":"sql injection"},
		{"checker":"sqli","class":"injection","severity":"high","message":"sql injection"},
		{"checker":"xss","class":"xss","severity":"medium","message":"reflected xss"}
	]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsDedup([]string{"--input", input, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, stdout.String())
	}
	if int(result["unique"].(float64)) != 2 {
		t.Errorf("unique = %v, want 2", result["unique"])
	}
	if int(result["duplicates"].(float64)) != 1 {
		t.Errorf("duplicates = %v, want 1", result["duplicates"])
	}
}

func TestFindingsDedupText(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "findings.json")
	os.WriteFile(input, []byte(`[
		{"checker":"a","class":"b","severity":"low","message":"m"},
		{"checker":"a","class":"b","severity":"low","message":"m"}
	]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsDedup([]string{"--input", input, "--format", "text"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	out := stdout.String()
	if !bytes.Contains([]byte(out), []byte("Unique:     1")) {
		t.Errorf("expected 'Unique:     1' in output, got: %s", out)
	}
}

func TestFindingsDedupWriteOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "findings.json")
	output := filepath.Join(dir, "deduped.json")
	os.WriteFile(input, []byte(`[
		{"checker":"a","class":"b","severity":"high","message":"x"},
		{"checker":"a","class":"b","severity":"high","message":"x"},
		{"checker":"c","class":"d","severity":"low","message":"y"}
	]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsDedup([]string{"--input", input, "--output", output, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var deduped []map[string]any
	if err := json.Unmarshal(raw, &deduped); err != nil {
		t.Fatalf("invalid output JSON: %v", err)
	}
	if len(deduped) != 2 {
		t.Errorf("deduped count = %d, want 2", len(deduped))
	}
}

func TestFindingsDedupMissingInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsDedup([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}

func TestFindingsDedupCustomKeys(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "findings.json")
	os.WriteFile(input, []byte(`[
		{"checker":"a","class":"b","severity":"high","message":"x"},
		{"checker":"a","class":"c","severity":"high","message":"x"}
	]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsDedup([]string{"--input", input, "--keys", "checker,severity", "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}

	var result map[string]any
	json.Unmarshal(stdout.Bytes(), &result)
	if int(result["unique"].(float64)) != 1 {
		t.Errorf("unique = %v, want 1 (same checker+severity)", result["unique"])
	}
}

func TestDedupHash(t *testing.T) {
	f1 := map[string]any{"a": "1", "b": "2"}
	f2 := map[string]any{"a": "1", "b": "2"}
	f3 := map[string]any{"a": "1", "b": "3"}

	h1 := dedupHash(f1, []string{"a", "b"})
	h2 := dedupHash(f2, []string{"a", "b"})
	h3 := dedupHash(f3, []string{"a", "b"})

	if h1 != h2 {
		t.Errorf("identical findings should have same hash")
	}
	if h1 == h3 {
		t.Errorf("different findings should have different hash")
	}
}
