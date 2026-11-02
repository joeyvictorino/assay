package modelcompare

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeRun(id, provider, modelName string) model.RunReport {
	return model.RunReport{
		RunID: id,
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Provider: provider, Model: modelName}},
		},
	}
}

func makeFinding(dedupKey, lab string, sev model.Severity, summary string) model.Finding {
	return model.Finding{
		ID:       dedupKey + "-id",
		DedupKey: dedupKey,
		Lab:      lab,
		Severity: sev,
		Summary:  summary,
	}
}

func TestCompareAllAgreed(t *testing.T) {
	a := RunData{
		Report:   makeRun("run-a", "openai", "gpt-4"),
		Findings: []model.Finding{makeFinding("f1", "lab1", "high", "SQL injection")},
	}
	b := RunData{
		Report:   makeRun("run-b", "anthropic", "claude-3"),
		Findings: []model.Finding{makeFinding("f1", "lab1", "high", "SQL injection found")},
	}
	r := Compare(a, b)
	if len(r.Agreed) != 1 {
		t.Fatalf("agreed=%d want 1", len(r.Agreed))
	}
	if len(r.OnlyA) != 0 || len(r.OnlyB) != 0 {
		t.Fatalf("only_a=%d only_b=%d want 0,0", len(r.OnlyA), len(r.OnlyB))
	}
	if r.AgreementRate != 1.0 {
		t.Fatalf("agreement_rate=%f want 1.0", r.AgreementRate)
	}
	if r.SeverityAlign.Matched != 1 || r.SeverityAlign.Mismatched != 0 {
		t.Fatalf("severity matched=%d mismatched=%d", r.SeverityAlign.Matched, r.SeverityAlign.Mismatched)
	}
}

func TestCompareDisagreement(t *testing.T) {
	a := RunData{
		Report: makeRun("run-a", "openai", "gpt-4"),
		Findings: []model.Finding{
			makeFinding("f1", "lab1", "high", "SQL injection"),
			makeFinding("f2", "lab1", "medium", "XSS"),
		},
	}
	b := RunData{
		Report: makeRun("run-b", "anthropic", "claude-3"),
		Findings: []model.Finding{
			makeFinding("f1", "lab1", "low", "SQL injection"),
			makeFinding("f3", "lab1", "critical", "RCE"),
		},
	}
	r := Compare(a, b)
	if len(r.Agreed) != 1 {
		t.Fatalf("agreed=%d want 1", len(r.Agreed))
	}
	if len(r.OnlyA) != 1 {
		t.Fatalf("only_a=%d want 1", len(r.OnlyA))
	}
	if len(r.OnlyB) != 1 {
		t.Fatalf("only_b=%d want 1", len(r.OnlyB))
	}
	if r.SeverityAlign.Mismatched != 1 {
		t.Fatalf("severity mismatched=%d want 1", r.SeverityAlign.Mismatched)
	}
	if r.ModelA != "openai/gpt-4" || r.ModelB != "anthropic/claude-3" {
		t.Fatalf("models: %s vs %s", r.ModelA, r.ModelB)
	}
}

func TestCompareEmpty(t *testing.T) {
	a := RunData{Report: makeRun("run-a", "", "m1")}
	b := RunData{Report: makeRun("run-b", "", "m2")}
	r := Compare(a, b)
	if r.AgreementRate != 0 || r.DisagreementRate != 0 {
		t.Fatalf("rates should be 0 for empty: agree=%f disagree=%f", r.AgreementRate, r.DisagreementRate)
	}
}

func TestCompareUsesIDWhenNoDedupKey(t *testing.T) {
	f := model.Finding{ID: "unique-id", Lab: "lab1", Severity: "high", Summary: "test"}
	a := RunData{
		Report:   makeRun("run-a", "", "m1"),
		Findings: []model.Finding{f},
	}
	b := RunData{
		Report:   makeRun("run-b", "", "m2"),
		Findings: []model.Finding{f},
	}
	r := Compare(a, b)
	if len(r.Agreed) != 1 {
		t.Fatalf("agreed=%d want 1", len(r.Agreed))
	}
}

func TestFormatTextOutput(t *testing.T) {
	a := RunData{
		Report:   makeRun("run-a", "openai", "gpt-4"),
		Findings: []model.Finding{makeFinding("f1", "lab1", "high", "SQL injection")},
	}
	b := RunData{
		Report: makeRun("run-b", "anthropic", "claude-3"),
	}
	r := Compare(a, b)
	text := FormatText(r)
	if text == "" {
		t.Fatal("empty text output")
	}
	if !contains(text, "openai/gpt-4") || !contains(text, "anthropic/claude-3") {
		t.Fatal("model names not in output")
	}
	if !contains(text, "Only in") {
		t.Fatal("expected 'Only in' section")
	}
}

func TestLoadRun(t *testing.T) {
	dir := t.TempDir()
	rr := makeRun("test-run", "test", "model")
	b, _ := json.Marshal(rr)
	p := filepath.Join(dir, "run.json")
	os.WriteFile(p, b, 0o644)
	rd, err := LoadRun(p)
	if err != nil {
		t.Fatal(err)
	}
	if rd.Report.RunID != "test-run" {
		t.Fatalf("run_id=%s want test-run", rd.Report.RunID)
	}
}

func TestLoadFindings(t *testing.T) {
	dir := t.TempDir()
	ff := []model.Finding{makeFinding("f1", "lab1", "high", "test")}
	b, _ := json.Marshal(ff)
	p := filepath.Join(dir, "findings.json")
	os.WriteFile(p, b, 0o644)
	loaded, err := LoadFindings(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].DedupKey != "f1" {
		t.Fatalf("loaded %v", loaded)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
