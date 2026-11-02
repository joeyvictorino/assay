package checkeragree

import (
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(dedupKey string, checker string, state model.FindingState) model.Finding {
	return model.Finding{
		ID:       dedupKey,
		DedupKey: dedupKey,
		Class:    model.ClassSQLi,
		Model:    model.ModelRef{Model: "gpt-4"},
		Validation: &model.ValidationResult{
			State:     state,
			Checker:   checker,
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

func TestSingleChecker(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("f1", "http-replay", model.StateValidated),
		finding("f2", "http-replay", model.StateRefuted),
	})
	if r.CheckerCount != 1 {
		t.Fatalf("want 1 checker, got %d", r.CheckerCount)
	}
	if len(r.Pairs) != 0 {
		t.Fatalf("want 0 pairs for single checker, got %d", len(r.Pairs))
	}
}

func TestTwoCheckersFullAgree(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("f1", "checker-a", model.StateValidated),
		finding("f1", "checker-b", model.StateValidated),
		finding("f2", "checker-a", model.StateRefuted),
		finding("f2", "checker-b", model.StateRefuted),
	})
	if r.CheckerCount != 2 {
		t.Fatalf("want 2 checkers, got %d", r.CheckerCount)
	}
	if len(r.Pairs) != 1 {
		t.Fatalf("want 1 pair, got %d", len(r.Pairs))
	}
	if r.Pairs[0].AgreeRate != 1.0 {
		t.Fatalf("want agree rate 1.0, got %.3f", r.Pairs[0].AgreeRate)
	}
	if r.Pairs[0].KappaScore != 1.0 {
		t.Fatalf("want kappa 1.0, got %.3f", r.Pairs[0].KappaScore)
	}
}

func TestTwoCheckersFullDisagree(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("f1", "checker-a", model.StateValidated),
		finding("f1", "checker-b", model.StateRefuted),
		finding("f2", "checker-a", model.StateRefuted),
		finding("f2", "checker-b", model.StateValidated),
	})
	if r.Pairs[0].AgreeRate != 0.0 {
		t.Fatalf("want agree rate 0.0, got %.3f", r.Pairs[0].AgreeRate)
	}
	if r.Pairs[0].KappaScore >= 0 {
		t.Fatalf("want negative kappa, got %.3f", r.Pairs[0].KappaScore)
	}
}

func TestThreeCheckers(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("f1", "a", model.StateValidated),
		finding("f1", "b", model.StateValidated),
		finding("f1", "c", model.StateRefuted),
	})
	if r.CheckerCount != 3 {
		t.Fatalf("want 3 checkers, got %d", r.CheckerCount)
	}
	if len(r.Pairs) != 3 {
		t.Fatalf("want 3 pairs, got %d", len(r.Pairs))
	}
}

func TestCheckerStats(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("f1", "checker-a", model.StateValidated),
		finding("f2", "checker-a", model.StateRefuted),
		finding("f3", "checker-a", model.StateValidated),
	})
	if len(r.Checkers) != 1 {
		t.Fatalf("want 1 checker stat, got %d", len(r.Checkers))
	}
	cs := r.Checkers[0]
	if cs.Validated != 2 {
		t.Fatalf("want 2 validated, got %d", cs.Validated)
	}
	if cs.Refuted != 1 {
		t.Fatalf("want 1 refuted, got %d", cs.Refuted)
	}
}

func TestCohensKappa(t *testing.T) {
	if cohensKappa(10, 0) != 1.0 {
		t.Fatal("perfect agree should be kappa=1.0")
	}
	if cohensKappa(0, 0) != 0 {
		t.Fatal("empty should be kappa=0")
	}
	k := cohensKappa(5, 5)
	if k != 0.0 {
		t.Fatalf("chance agree should be kappa=0, got %.3f", k)
	}
}

func TestNoValidation(t *testing.T) {
	r := Analyze([]model.Finding{
		{ID: "f1", Class: model.ClassSQLi},
	})
	if r.CheckerCount != 0 {
		t.Fatalf("want 0 checkers for unchecked findings, got %d", r.CheckerCount)
	}
}

func TestPoorAgreementRecommendation(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("f1", "a", model.StateValidated),
		finding("f1", "b", model.StateRefuted),
	})
	if !strings.Contains(r.Recommendation, "poor") {
		t.Fatalf("should mention poor agreement, got %q", r.Recommendation)
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("f1", "checker-a", model.StateValidated),
	})
	text := FormatText(r)
	if !strings.Contains(text, "Inter-Checker") {
		t.Fatal("should contain header")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("f1", "checker-a", model.StateValidated),
	})
	j := FormatJSON(r)
	if !strings.Contains(j, "overall_kappa") {
		t.Fatal("should contain overall_kappa field")
	}
}
