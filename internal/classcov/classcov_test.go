package classcov

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(cls model.Class, sev model.Severity, mdl string) model.Finding {
	return model.Finding{
		ID:       "f1",
		Class:    cls,
		Severity: sev,
		Model:    model.ModelRef{Model: mdl},
	}
}

func TestEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.CoveredClasses != 0 {
		t.Fatalf("want 0 covered, got %d", r.CoveredClasses)
	}
	if r.TotalClasses != len(model.AllClasses) {
		t.Fatalf("want %d total, got %d", len(model.AllClasses), r.TotalClasses)
	}
	if len(r.MissingClasses) != len(model.AllClasses) {
		t.Fatalf("want all classes missing, got %d", len(r.MissingClasses))
	}
}

func TestSingleClass(t *testing.T) {
	r := Analyze([]model.Finding{finding(model.ClassSQLi, model.SevHigh, "gpt-4")})
	if r.CoveredClasses != 1 {
		t.Fatalf("want 1 covered, got %d", r.CoveredClasses)
	}
	if len(r.MissingClasses) != len(model.AllClasses)-1 {
		t.Fatalf("want %d missing, got %d", len(model.AllClasses)-1, len(r.MissingClasses))
	}
}

func TestMultipleClasses(t *testing.T) {
	r := Analyze([]model.Finding{
		finding(model.ClassSQLi, model.SevHigh, "gpt-4"),
		finding(model.ClassXSSReflected, model.SevMedium, "gpt-4"),
		finding(model.ClassIDOR, model.SevLow, "gpt-4"),
	})
	if r.CoveredClasses != 3 {
		t.Fatalf("want 3 covered, got %d", r.CoveredClasses)
	}
}

func TestMaxSeverity(t *testing.T) {
	r := Analyze([]model.Finding{
		finding(model.ClassSQLi, model.SevLow, "gpt-4"),
		finding(model.ClassSQLi, model.SevCritical, "gpt-4"),
		finding(model.ClassSQLi, model.SevMedium, "gpt-4"),
	})
	if len(r.ByClass) != 1 {
		t.Fatalf("want 1 class, got %d", len(r.ByClass))
	}
	if r.ByClass[0].MaxSeverity != "critical" {
		t.Fatalf("want max critical, got %s", r.ByClass[0].MaxSeverity)
	}
}

func TestByModel(t *testing.T) {
	r := Analyze([]model.Finding{
		finding(model.ClassSQLi, model.SevHigh, "gpt-4"),
		finding(model.ClassXSSReflected, model.SevHigh, "gpt-4"),
		finding(model.ClassSQLi, model.SevHigh, "claude-3"),
	})
	if len(r.ByModel) != 2 {
		t.Fatalf("want 2 models, got %d", len(r.ByModel))
	}
	if r.ByModel[0].Covered < r.ByModel[1].Covered {
		t.Fatal("models should be sorted by coverage descending")
	}
}

func TestCoveragePct(t *testing.T) {
	r := Analyze([]model.Finding{finding(model.ClassSQLi, model.SevHigh, "gpt-4")})
	expected := 1.0 / float64(len(model.AllClasses))
	if r.CoveragePct < expected-0.01 || r.CoveragePct > expected+0.01 {
		t.Fatalf("want ~%.3f coverage, got %.3f", expected, r.CoveragePct)
	}
}

func TestLowCoverageRecommendation(t *testing.T) {
	r := Analyze([]model.Finding{finding(model.ClassSQLi, model.SevHigh, "gpt-4")})
	if !strings.Contains(r.Recommendation, "low coverage") {
		t.Fatalf("should mention low coverage, got %q", r.Recommendation)
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze([]model.Finding{finding(model.ClassSQLi, model.SevHigh, "gpt-4")})
	text := FormatText(r)
	if !strings.Contains(text, "Vulnerability Class Coverage") {
		t.Fatal("should contain header")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding(model.ClassSQLi, model.SevHigh, "gpt-4")})
	j := FormatJSON(r)
	if !strings.Contains(j, "coverage_pct") {
		t.Fatal("should contain coverage_pct field")
	}
}
