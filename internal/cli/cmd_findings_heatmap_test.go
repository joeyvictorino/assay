package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindingsHeatmapMissingInput(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runFindingsHeatmap([]string{}, &out, &errOut)
	if code != ExitError {
		t.Fatalf("expected ExitError, got %d", code)
	}
	if !strings.Contains(errOut.String(), "--input is required") {
		t.Fatalf("expected input required message, got %q", errOut.String())
	}
}

func TestFindingsHeatmapEmpty(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "findings.json")
	os.WriteFile(f, []byte("[]"), 0644)

	var out, errOut bytes.Buffer
	code := runFindingsHeatmap([]string{"--input", f}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "No findings") {
		t.Fatalf("expected 'No findings' in output, got %q", out.String())
	}
}

func TestFindingsHeatmapText(t *testing.T) {
	findings := []map[string]any{
		{"checker": "auth", "severity": "high", "message": "no auth"},
		{"checker": "auth", "severity": "high", "message": "weak auth"},
		{"checker": "auth", "severity": "medium", "message": "session"},
		{"checker": "network", "severity": "critical", "message": "open port"},
		{"checker": "network", "severity": "low", "message": "info"},
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "findings.json")
	data, _ := json.Marshal(findings)
	os.WriteFile(f, data, 0644)

	var out, errOut bytes.Buffer
	code := runFindingsHeatmap([]string{"--input", f}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr=%s", code, errOut.String())
	}
	text := out.String()
	if !strings.Contains(text, "Findings Heatmap (5 total)") {
		t.Fatalf("expected heatmap header, got %q", text)
	}
	if !strings.Contains(text, "auth") {
		t.Fatalf("expected 'auth' checker in output, got %q", text)
	}
	if !strings.Contains(text, "network") {
		t.Fatalf("expected 'network' checker in output, got %q", text)
	}
}

func TestFindingsHeatmapJSON(t *testing.T) {
	findings := []map[string]any{
		{"checker": "c1", "severity": "high"},
		{"checker": "c1", "severity": "low"},
		{"checker": "c2", "severity": "high"},
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "findings.json")
	data, _ := json.Marshal(findings)
	os.WriteFile(f, data, 0644)

	var out, errOut bytes.Buffer
	code := runFindingsHeatmap([]string{"--input", f, "--format", "json"}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr=%s", code, errOut.String())
	}

	var res heatmapResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if res.Total != 3 {
		t.Fatalf("expected total 3, got %d", res.Total)
	}
	if len(res.Checkers) != 2 {
		t.Fatalf("expected 2 checkers, got %d", len(res.Checkers))
	}
	if res.Grid["c1"]["high"] != 1 {
		t.Fatalf("expected c1/high=1, got %d", res.Grid["c1"]["high"])
	}
	if res.Grid["c1"]["low"] != 1 {
		t.Fatalf("expected c1/low=1, got %d", res.Grid["c1"]["low"])
	}
}

func TestFindingsHeatmapUnknownFields(t *testing.T) {
	findings := []map[string]any{
		{"message": "no checker or severity"},
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "findings.json")
	data, _ := json.Marshal(findings)
	os.WriteFile(f, data, 0644)

	var out, errOut bytes.Buffer
	code := runFindingsHeatmap([]string{"--input", f, "--format", "json"}, &out, &errOut)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d", code)
	}
	var res heatmapResult
	json.Unmarshal(out.Bytes(), &res)
	if res.Grid["(unknown)"]["(unknown)"] != 1 {
		t.Fatalf("expected unknown/unknown=1")
	}
}
