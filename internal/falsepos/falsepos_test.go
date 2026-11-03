package falsepos

import (
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func validated(cls model.Class, mdl string) model.Finding {
	return model.Finding{
		ID:       "f1",
		DedupKey: string(cls) + "|" + mdl,
		Class:    cls,
		Model:    model.ModelRef{Model: mdl},
		Validation: &model.ValidationResult{
			State:   model.StateValidated,
			Checker: "http-replay",
			CheckedAt: time.Now(),
		},
	}
}

func refuted(cls model.Class, mdl string) model.Finding {
	return model.Finding{
		ID:       "f2",
		DedupKey: string(cls) + "|" + mdl,
		Class:    cls,
		Model:    model.ModelRef{Model: mdl},
		Validation: &model.ValidationResult{
			State:   model.StateRefuted,
			Checker: "http-replay",
			CheckedAt: time.Now(),
		},
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

func TestAllValidated(t *testing.T) {
	r := Analyze([]model.Finding{
		validated(model.ClassSQLi, "gpt-4"),
		validated(model.ClassXSSReflected, "gpt-4"),
	})
	if r.OverallFPRate != 0.0 {
		t.Fatalf("want FP rate 0.0, got %.3f", r.OverallFPRate)
	}
	if r.ValidatedCount != 2 {
		t.Fatalf("want 2 validated, got %d", r.ValidatedCount)
	}
}

func TestAllRefuted(t *testing.T) {
	r := Analyze([]model.Finding{
		refuted(model.ClassSQLi, "gpt-4"),
		refuted(model.ClassXSSReflected, "gpt-4"),
	})
	if r.OverallFPRate != 1.0 {
		t.Fatalf("want FP rate 1.0, got %.3f", r.OverallFPRate)
	}
	if r.RefutedCount != 2 {
		t.Fatalf("want 2 refuted, got %d", r.RefutedCount)
	}
}

func TestMixed(t *testing.T) {
	r := Analyze([]model.Finding{
		validated(model.ClassSQLi, "gpt-4"),
		refuted(model.ClassSQLi, "gpt-4"),
		validated(model.ClassSQLi, "gpt-4"),
		refuted(model.ClassSQLi, "gpt-4"),
	})
	if r.OverallFPRate != 0.5 {
		t.Fatalf("want FP rate 0.5, got %.3f", r.OverallFPRate)
	}
}

func TestByClass(t *testing.T) {
	r := Analyze([]model.Finding{
		validated(model.ClassSQLi, "gpt-4"),
		refuted(model.ClassXSSReflected, "gpt-4"),
	})
	if len(r.ByClass) != 2 {
		t.Fatalf("want 2 classes, got %d", len(r.ByClass))
	}
	for _, cr := range r.ByClass {
		if cr.Class == "sqli" && cr.FPRate != 0.0 {
			t.Fatalf("sqli should have 0 FP rate, got %.3f", cr.FPRate)
		}
		if cr.Class == "xss-reflected" && cr.FPRate != 1.0 {
			t.Fatalf("xss should have 1.0 FP rate, got %.3f", cr.FPRate)
		}
	}
}

func TestByModel(t *testing.T) {
	r := Analyze([]model.Finding{
		validated(model.ClassSQLi, "gpt-4"),
		refuted(model.ClassSQLi, "claude-3"),
	})
	if len(r.ByModel) != 2 {
		t.Fatalf("want 2 models, got %d", len(r.ByModel))
	}
}

func TestNoValidation(t *testing.T) {
	r := Analyze([]model.Finding{
		{ID: "f1", Class: model.ClassSQLi},
	})
	if r.ValidatedCount != 0 {
		t.Fatalf("want 0 validated, got %d", r.ValidatedCount)
	}
	if r.OverallFPRate != 0 {
		t.Fatalf("want 0 FP rate for unchecked, got %.3f", r.OverallFPRate)
	}
}

func TestConfidenceInterval(t *testing.T) {
	r := Analyze([]model.Finding{
		refuted(model.ClassSQLi, "gpt-4"),
		validated(model.ClassSQLi, "gpt-4"),
	})
	if len(r.ByClass) == 0 {
		t.Fatal("want at least one class")
	}
	ci := r.ByClass[0].ConfInterval
	if ci[0] >= ci[1] {
		t.Fatalf("confidence interval lower >= upper: [%.3f, %.3f]", ci[0], ci[1])
	}
}

func TestHighFPRecommendation(t *testing.T) {
	ff := make([]model.Finding, 0, 10)
	for i := 0; i < 8; i++ {
		ff = append(ff, refuted(model.ClassSQLi, "gpt-4"))
	}
	ff = append(ff, validated(model.ClassSQLi, "gpt-4"))
	ff = append(ff, validated(model.ClassSQLi, "gpt-4"))
	r := Analyze(ff)
	if !strings.Contains(r.Recommendation, "tuning") {
		t.Fatalf("should recommend tuning for high FP, got %q", r.Recommendation)
	}
}

func TestWilsonInterval(t *testing.T) {
	lo, hi := wilsonInterval(100, 50, 1.96)
	if lo >= 0.5 || hi <= 0.5 {
		t.Fatalf("wilson interval for 50/100 should contain 0.5, got [%.3f, %.3f]", lo, hi)
	}
	lo0, hi0 := wilsonInterval(0, 0, 1.96)
	if lo0 != 0 || hi0 != 1 {
		t.Fatalf("wilson for 0/0 should be [0, 1], got [%.3f, %.3f]", lo0, hi0)
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze([]model.Finding{validated(model.ClassSQLi, "gpt-4")})
	text := FormatText(r)
	if !strings.Contains(text, "False-Positive") {
		t.Fatal("should contain header")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{validated(model.ClassSQLi, "gpt-4")})
	j := FormatJSON(r)
	if !strings.Contains(j, "overall_fp_rate") {
		t.Fatal("should contain overall_fp_rate field")
	}
}
