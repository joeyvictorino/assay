package classcov

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func f(mdl string, cls model.Class) model.Finding {
	return model.Finding{
		Model: model.ModelRef{Model: mdl},
		Class: cls,
	}
}

func TestEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.TotalFindings != 0 {
		t.Fatalf("want 0, got %d", r.TotalFindings)
	}
	if r.Recommendation != "No findings to analyze" {
		t.Fatalf("want no-findings recommendation, got %q", r.Recommendation)
	}
}

func TestSingleFinding(t *testing.T) {
	r := Analyze([]model.Finding{f("gpt-4", model.ClassSQLi)})
	if r.TotalClasses != 1 {
		t.Fatalf("want 1 class, got %d", r.TotalClasses)
	}
	if r.TotalModels != 1 {
		t.Fatalf("want 1 model, got %d", r.TotalModels)
	}
}

func TestMultipleClasses(t *testing.T) {
	r := Analyze([]model.Finding{
		f("gpt-4", model.ClassSQLi),
		f("gpt-4", model.ClassXSSReflected),
		f("gpt-4", model.ClassIDOR),
	})
	if r.TotalClasses != 3 {
		t.Fatalf("want 3 classes, got %d", r.TotalClasses)
	}
}

func TestMultipleModels(t *testing.T) {
	r := Analyze([]model.Finding{
		f("gpt-4", model.ClassSQLi),
		f("claude-3", model.ClassSQLi),
	})
	if r.TotalModels != 2 {
		t.Fatalf("want 2 models, got %d", r.TotalModels)
	}
	if len(r.ByClass) != 1 {
		t.Fatalf("want 1 class, got %d", len(r.ByClass))
	}
	if r.ByClass[0].ModelCount != 2 {
		t.Fatalf("want 2 models for sqli, got %d", r.ByClass[0].ModelCount)
	}
}

func TestModelCoverage(t *testing.T) {
	r := Analyze([]model.Finding{
		f("gpt-4", model.ClassSQLi),
		f("gpt-4", model.ClassXSSReflected),
	})
	if len(r.ByModel) != 1 {
		t.Fatalf("want 1 model, got %d", len(r.ByModel))
	}
	if r.ByModel[0].ClassCount != 2 {
		t.Fatalf("want 2 classes for gpt-4, got %d", r.ByModel[0].ClassCount)
	}
}

func TestUndetectedClasses(t *testing.T) {
	r := Analyze([]model.Finding{f("gpt-4", model.ClassSQLi)})
	if len(r.UndetectedClasses) != len(model.AllClasses)-1 {
		t.Fatalf("want %d undetected, got %d", len(model.AllClasses)-1, len(r.UndetectedClasses))
	}
}

func TestCoverageByPct(t *testing.T) {
	r := Analyze([]model.Finding{
		f("gpt-4", model.ClassSQLi),
		f("claude-3", model.ClassSQLi),
	})
	if r.ByClass[0].CoverageBy != 100.0 {
		t.Fatalf("want 100%% coverage_by for sqli, got %.1f%%", r.ByClass[0].CoverageBy)
	}
}

func TestRecommendation(t *testing.T) {
	r := Analyze([]model.Finding{f("gpt-4", model.ClassSQLi)})
	if !strings.Contains(r.Recommendation, "1/") {
		t.Fatalf("recommendation should mention class count, got %q", r.Recommendation)
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze([]model.Finding{f("gpt-4", model.ClassSQLi)})
	text := FormatText(r)
	if !strings.Contains(text, "Class Coverage") {
		t.Fatal("should contain header")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{f("gpt-4", model.ClassSQLi)})
	j := FormatJSON(r)
	if !strings.Contains(j, "undetected_classes") {
		t.Fatal("should contain undetected_classes field")
	}
}

func TestClassSorted(t *testing.T) {
	r := Analyze([]model.Finding{
		f("gpt-4", model.ClassSQLi),
		f("gpt-4", model.ClassSQLi),
		f("gpt-4", model.ClassXSSReflected),
	})
	if r.ByClass[0].Class != "sqli" {
		t.Fatalf("want sqli first (most findings), got %s", r.ByClass[0].Class)
	}
}
