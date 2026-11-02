package calibrate

import (
	"math"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestPearsonPerfectCorrelation(t *testing.T) {
	pp := []scorePair{{1, 1}, {2, 2}, {3, 3}, {4, 4}}
	r := pearson(pp)
	if math.Abs(r-1.0) > 0.001 {
		t.Errorf("pearson = %.4f, want 1.0", r)
	}
}

func TestPearsonNegativeCorrelation(t *testing.T) {
	pp := []scorePair{{1, 4}, {2, 3}, {3, 2}, {4, 1}}
	r := pearson(pp)
	if math.Abs(r-(-1.0)) > 0.001 {
		t.Errorf("pearson = %.4f, want -1.0", r)
	}
}

func TestPearsonSingleSample(t *testing.T) {
	pp := []scorePair{{5, 3}}
	r := pearson(pp)
	if r != 0 {
		t.Errorf("pearson with 1 sample = %.4f, want 0", r)
	}
}

func TestPearsonConstant(t *testing.T) {
	pp := []scorePair{{3, 3}, {3, 3}, {3, 3}}
	r := pearson(pp)
	if r != 0 {
		t.Errorf("pearson with constant values = %.4f, want 0", r)
	}
}

func TestRateExcellent(t *testing.T) {
	got := rate(0.05, 0.9)
	if got != "excellent" {
		t.Errorf("rate(0.05, 0.9) = %q, want excellent", got)
	}
}

func TestRateGood(t *testing.T) {
	got := rate(0.15, 0.7)
	if got != "good" {
		t.Errorf("rate(0.15, 0.7) = %q, want good", got)
	}
}

func TestRateFair(t *testing.T) {
	got := rate(0.25, 0.3)
	if got != "fair" {
		t.Errorf("rate(0.25, 0.3) = %q, want fair", got)
	}
}

func TestRatePoor(t *testing.T) {
	got := rate(0.5, 0.1)
	if got != "poor" {
		t.Errorf("rate(0.5, 0.1) = %q, want poor", got)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	rr := model.RunReport{}
	report := Analyze(rr)
	if len(report.Checkers) != 0 {
		t.Errorf("expected 0 checkers, got %d", len(report.Checkers))
	}
}

func TestAnalyzeWithPairs(t *testing.T) {
	rr := model.RunReport{
		RunID: "test-run",
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "gpt-4o", ReviewerKind: "self", Scored: true, Final: 8},
			{Lab: "lab1", Model: "gpt-4o", ReviewerKind: "peer", Scored: true, Final: 7},
			{Lab: "lab2", Model: "gpt-4o", ReviewerKind: "self", Scored: true, Final: 6},
			{Lab: "lab2", Model: "gpt-4o", ReviewerKind: "peer", Scored: true, Final: 5},
		},
	}
	report := Analyze(rr)
	if len(report.Checkers) != 1 {
		t.Fatalf("expected 1 checker, got %d", len(report.Checkers))
	}
	c := report.Checkers[0]
	if c.Checker != "gpt-4o" {
		t.Errorf("checker = %q, want gpt-4o", c.Checker)
	}
	if c.SampleCount != 2 {
		t.Errorf("sample count = %d, want 2", c.SampleCount)
	}
	if c.Bias != 1.0 {
		t.Errorf("bias = %.2f, want 1.0", c.Bias)
	}
	if c.AbsError != 1.0 {
		t.Errorf("abs error = %.2f, want 1.0", c.AbsError)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	rr := model.RunReport{
		RunID: "test-run",
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "gpt-4o", ReviewerKind: "self", Scored: true, Final: 8},
			{Lab: "lab1", Model: "gpt-4o", ReviewerKind: "peer", Scored: true, Final: 7},
			{Lab: "lab1", Model: "claude", ReviewerKind: "self", Scored: true, Final: 9},
			{Lab: "lab1", Model: "claude", ReviewerKind: "peer", Scored: true, Final: 9},
		},
	}
	report := Analyze(rr)
	if len(report.Checkers) != 2 {
		t.Fatalf("expected 2 checkers, got %d", len(report.Checkers))
	}
	if report.Overall.BestChecker != "claude" {
		t.Errorf("best checker = %q, want claude", report.Overall.BestChecker)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := CalibrationReport{}
	text := FormatText(r)
	if text == "" {
		t.Error("expected non-empty text for empty report")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := CalibrationReport{
		Checkers: []CheckerCalibration{
			{Checker: "gpt-4o", SampleCount: 5, MeanSelfScore: 7.5, MeanPeerScore: 7.0, Bias: 0.5, AbsError: 0.5, Correlation: 0.9, Rating: "good"},
		},
		Overall: OverallCalibration{MeanBias: 0.5, MeanAbsError: 0.5, BestChecker: "gpt-4o", WorstChecker: "gpt-4o"},
	}
	text := FormatText(r)
	if text == "" {
		t.Error("expected non-empty text")
	}
}
