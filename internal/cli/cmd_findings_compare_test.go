package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func writeTempFindings(t *testing.T, findings []map[string]any) string {
	t.Helper()
	data, _ := json.Marshal(findings)
	f, err := os.CreateTemp(t.TempDir(), "findings-*.json")
	if err != nil {
		t.Fatal(err)
	}
	f.Write(data)
	f.Close()
	return f.Name()
}

func TestFindingsCompareIdentical(t *testing.T) {
	findings := []map[string]any{
		{"checker": "c1", "class": "xss", "severity": "high", "message": "xss found"},
	}
	a := writeTempFindings(t, findings)
	b := writeTempFindings(t, findings)

	var out bytes.Buffer
	code := runFindingsCompare([]string{"--a", a, "--b", b, "--format", "json"}, &out, &bytes.Buffer{})
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}

	var result findingsDiff
	json.Unmarshal(out.Bytes(), &result)
	if result.Common != 1 {
		t.Fatalf("expected 1 common, got %d", result.Common)
	}
	if result.AddedCount != 0 || result.RemovedCount != 0 {
		t.Fatalf("expected no changes, got added=%d removed=%d", result.AddedCount, result.RemovedCount)
	}
}

func TestFindingsCompareAdded(t *testing.T) {
	a := writeTempFindings(t, []map[string]any{
		{"checker": "c1", "class": "xss", "severity": "high", "message": "xss"},
	})
	b := writeTempFindings(t, []map[string]any{
		{"checker": "c1", "class": "xss", "severity": "high", "message": "xss"},
		{"checker": "c2", "class": "sqli", "severity": "critical", "message": "sqli"},
	})

	var out bytes.Buffer
	code := runFindingsCompare([]string{"--a", a, "--b", b, "--format", "json"}, &out, &bytes.Buffer{})
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}

	var result findingsDiff
	json.Unmarshal(out.Bytes(), &result)
	if result.AddedCount != 1 {
		t.Fatalf("expected 1 added, got %d", result.AddedCount)
	}
}

func TestFindingsCompareRemoved(t *testing.T) {
	a := writeTempFindings(t, []map[string]any{
		{"checker": "c1", "class": "xss", "severity": "high", "message": "xss"},
		{"checker": "c2", "class": "sqli", "severity": "critical", "message": "sqli"},
	})
	b := writeTempFindings(t, []map[string]any{
		{"checker": "c1", "class": "xss", "severity": "high", "message": "xss"},
	})

	var out bytes.Buffer
	code := runFindingsCompare([]string{"--a", a, "--b", b, "--format", "json"}, &out, &bytes.Buffer{})
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}

	var result findingsDiff
	json.Unmarshal(out.Bytes(), &result)
	if result.RemovedCount != 1 {
		t.Fatalf("expected 1 removed, got %d", result.RemovedCount)
	}
}

func TestFindingsCompareText(t *testing.T) {
	a := writeTempFindings(t, []map[string]any{
		{"checker": "c1", "class": "xss", "severity": "high", "message": "xss"},
	})
	b := writeTempFindings(t, []map[string]any{
		{"checker": "c2", "class": "sqli", "severity": "critical", "message": "sqli"},
	})

	var out bytes.Buffer
	code := runFindingsCompare([]string{"--a", a, "--b", b}, &out, &bytes.Buffer{})
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}
	text := out.String()
	if !strings.Contains(text, "Findings Comparison") {
		t.Fatal("expected header in text output")
	}
	if !strings.Contains(text, "Added") || !strings.Contains(text, "Removed") {
		t.Fatal("expected Added/Removed labels")
	}
}

func TestFindingsCompareMissingFlags(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runFindingsCompare(nil, &out, &errOut)
	if code != ExitError {
		t.Fatalf("expected error exit, got %d", code)
	}
}

func TestFindingsCompareEmpty(t *testing.T) {
	a := writeTempFindings(t, []map[string]any{})
	b := writeTempFindings(t, []map[string]any{})

	var out bytes.Buffer
	code := runFindingsCompare([]string{"--a", a, "--b", b, "--format", "json"}, &out, &bytes.Buffer{})
	if code != ExitPass {
		t.Fatalf("exit %d", code)
	}

	var result findingsDiff
	json.Unmarshal(out.Bytes(), &result)
	if result.Common != 0 || result.AddedCount != 0 || result.RemovedCount != 0 {
		t.Fatal("expected all zeros for empty files")
	}
}
