package promptevolve

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(id, lab, runID string, sev model.Severity) model.Finding {
	return model.Finding{
		ID:       id,
		Lab:      lab,
		RunID:    runID,
		Severity: sev,
		Summary:  "test finding " + id,
		Model:    model.ModelRef{Provider: "test", Model: "test-model"},
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.Summary.TotalPrompts != 0 {
		t.Fatalf("expected 0, got %d", r.Summary.TotalPrompts)
	}
}

func TestAnalyzeSingleLab(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "injection-lab", "run1", model.SevHigh),
		makeFinding("f2", "injection-lab", "run1", model.SevMedium),
	}
	r := Analyze(findings)
	if r.Summary.TotalPrompts != 1 {
		t.Fatalf("expected 1 prompt, got %d", r.Summary.TotalPrompts)
	}
}

func TestAnalyzeMultipleVersions(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "injection-lab", "run1", model.SevHigh),
		makeFinding("f2", "injection-lab", "run2", model.SevHigh),
		makeFinding("f3", "injection-lab", "run2", model.SevMedium),
	}
	r := Analyze(findings)
	if r.Summary.TotalVersions < 2 {
		t.Fatalf("expected at least 2 versions, got %d", r.Summary.TotalVersions)
	}
}

func TestAnalyzeMultipleLabs(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "lab-a", "run1", model.SevHigh),
		makeFinding("f2", "lab-b", "run1", model.SevLow),
	}
	r := Analyze(findings)
	if r.Summary.TotalPrompts != 2 {
		t.Fatalf("expected 2 prompts, got %d", r.Summary.TotalPrompts)
	}
}

func TestDefaultLabUsed(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "", "run1", model.SevMedium),
	}
	r := Analyze(findings)
	if r.Evolutions[0].Name != "default" {
		t.Fatalf("expected 'default' lab, got %q", r.Evolutions[0].Name)
	}
}

func TestFormatTextContainsHeader(t *testing.T) {
	r := Analyze(nil)
	out := FormatText(r)
	if !strings.Contains(out, "Prompt Evolution Tracker") {
		t.Fatal("expected header")
	}
}

func TestFormatJSONContainsKey(t *testing.T) {
	findings := []model.Finding{makeFinding("f1", "test", "run1", model.SevHigh)}
	r := Analyze(findings)
	out := FormatJSON(r)
	if !strings.Contains(out, `"total_prompts"`) {
		t.Fatal("expected total_prompts in JSON")
	}
}

func TestSeverityScore(t *testing.T) {
	if severityScore(model.SevCritical) != 5.0 {
		t.Fatal("critical should be 5.0")
	}
	if severityScore(model.SevInfo) != 1.0 {
		t.Fatal("info should be 1.0")
	}
}
