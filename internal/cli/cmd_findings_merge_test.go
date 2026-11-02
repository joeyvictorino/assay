package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFindingsMergeBasic(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.json")
	f2 := filepath.Join(dir, "b.json")
	os.WriteFile(f1, []byte(`[{"checker":"sqli","class":"inj","severity":"high","message":"a"}]`), 0o644)
	os.WriteFile(f2, []byte(`[{"checker":"xss","class":"xss","severity":"medium","message":"b"}]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsMerge([]string{"--inputs", f1 + "," + f2, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}
	var result map[string]any
	json.Unmarshal(stdout.Bytes(), &result)
	if int(result["total_after"].(float64)) != 2 {
		t.Errorf("total_after = %v, want 2", result["total_after"])
	}
}

func TestFindingsMergeDedup(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.json")
	f2 := filepath.Join(dir, "b.json")
	os.WriteFile(f1, []byte(`[{"checker":"sqli","class":"inj","severity":"high","message":"dup"}]`), 0o644)
	os.WriteFile(f2, []byte(`[{"checker":"sqli","class":"inj","severity":"high","message":"dup"}]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsMerge([]string{"--inputs", f1 + "," + f2, "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	var result map[string]any
	json.Unmarshal(stdout.Bytes(), &result)
	if int(result["total_before"].(float64)) != 2 {
		t.Errorf("total_before = %v, want 2", result["total_before"])
	}
	if int(result["total_after"].(float64)) != 1 {
		t.Errorf("total_after = %v, want 1 (dedup)", result["total_after"])
	}
}

func TestFindingsMergeNoDedup(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.json")
	os.WriteFile(f1, []byte(`[{"checker":"a","class":"b","severity":"c","message":"d"},{"checker":"a","class":"b","severity":"c","message":"d"}]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsMerge([]string{"--inputs", f1, "--dedup=false", "--format", "json"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	var result map[string]any
	json.Unmarshal(stdout.Bytes(), &result)
	if int(result["total_after"].(float64)) != 2 {
		t.Errorf("total_after = %v, want 2 (no dedup)", result["total_after"])
	}
}

func TestFindingsMergeText(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.json")
	os.WriteFile(f1, []byte(`[{"checker":"a","severity":"low","message":"m"}]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsMerge([]string{"--inputs", f1, "--format", "text"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Sources:")) {
		t.Errorf("expected 'Sources:' in output, got: %s", stdout.String())
	}
}

func TestFindingsMergeWriteOutput(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.json")
	out := filepath.Join(dir, "merged.json")
	os.WriteFile(f1, []byte(`[{"checker":"a","severity":"low","message":"m"}]`), 0o644)

	var stdout, stderr bytes.Buffer
	code := runFindingsMerge([]string{"--inputs", f1, "--output", out, "--format", "text"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	raw, _ := os.ReadFile(out)
	var merged []map[string]any
	if err := json.Unmarshal(raw, &merged); err != nil {
		t.Fatalf("output not valid JSON: %v", err)
	}
	if len(merged) != 1 {
		t.Errorf("merged count = %d, want 1", len(merged))
	}
}

func TestFindingsMergeMissingInputs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runFindingsMerge([]string{}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError, got %d", code)
	}
}
