package sensitivity

import (
	"math"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyzeEmpty(t *testing.T) {
	rr := model.RunReport{}
	r := Analyze(rr)
	if len(r.Checkers) != 0 {
		t.Errorf("expected 0 checkers, got %d", len(r.Checkers))
	}
}

func TestAnalyzeSingleVariant(t *testing.T) {
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "m1", Reviewer: "checker-a", Scored: true, Final: 8},
		},
	}
	r := Analyze(rr)
	if len(r.Checkers) != 1 {
		t.Fatalf("expected 1 checker, got %d", len(r.Checkers))
	}
	if r.Checkers[0].Rating != "stable" {
		t.Errorf("single variant should be stable, got %s", r.Checkers[0].Rating)
	}
}

func TestAnalyzeMultipleVariants(t *testing.T) {
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "m1", Reviewer: "checker-a", PromptVariant: "v1", Scored: true, Final: 8},
			{Lab: "lab1", Model: "m1", Reviewer: "checker-a", PromptVariant: "v2", Scored: true, Final: 6},
			{Lab: "lab1", Model: "m1", Reviewer: "checker-a", PromptVariant: "v3", Scored: true, Final: 7},
		},
	}
	r := Analyze(rr)
	if len(r.Checkers) != 1 {
		t.Fatalf("expected 1 checker, got %d", len(r.Checkers))
	}
	c := r.Checkers[0]
	if c.MeanScore != 7.0 {
		t.Errorf("mean = %.2f, want 7.0", c.MeanScore)
	}
	if c.Range != 2.0 {
		t.Errorf("range = %.2f, want 2.0", c.Range)
	}
}

func TestRateCV(t *testing.T) {
	tests := []struct {
		cv   float64
		want string
	}{
		{0.05, "stable"},
		{0.15, "moderate"},
		{0.25, "sensitive"},
		{0.35, "highly_sensitive"},
	}
	for _, tt := range tests {
		got := rateCV(tt.cv)
		if got != tt.want {
			t.Errorf("rateCV(%.2f) = %s, want %s", tt.cv, got, tt.want)
		}
	}
}

func TestComputeSensitivityStdDev(t *testing.T) {
	variants := []PromptVariant{
		{Score: 10}, {Score: 10}, {Score: 10},
	}
	cs := computeSensitivity("test", variants)
	if cs.StdDev != 0 {
		t.Errorf("stddev = %.2f, want 0", cs.StdDev)
	}
	if cs.CV != 0 {
		t.Errorf("cv = %.4f, want 0", cs.CV)
	}
}

func TestComputeSensitivityRange(t *testing.T) {
	variants := []PromptVariant{
		{Score: 2}, {Score: 8},
	}
	cs := computeSensitivity("test", variants)
	if cs.Range != 6.0 {
		t.Errorf("range = %.2f, want 6.0", cs.Range)
	}
	if cs.MeanScore != 5.0 {
		t.Errorf("mean = %.2f, want 5.0", cs.MeanScore)
	}
}

func TestMostStableMostSensitive(t *testing.T) {
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "m1", Reviewer: "stable-checker", PromptVariant: "v1", Scored: true, Final: 10},
			{Lab: "lab1", Model: "m1", Reviewer: "stable-checker", PromptVariant: "v2", Scored: true, Final: 10},
			{Lab: "lab1", Model: "m1", Reviewer: "wild-checker", PromptVariant: "v1", Scored: true, Final: 2},
			{Lab: "lab1", Model: "m1", Reviewer: "wild-checker", PromptVariant: "v2", Scored: true, Final: 10},
		},
	}
	r := Analyze(rr)
	if r.MostStable != "stable-checker" {
		t.Errorf("most stable = %s, want stable-checker", r.MostStable)
	}
	if r.MostSensitive != "wild-checker" {
		t.Errorf("most sensitive = %s, want wild-checker", r.MostSensitive)
	}
}

func TestFormatText(t *testing.T) {
	r := SensitivityReport{
		Checkers: []CheckerSensitivity{
			{Checker: "c1", MeanScore: 7.5, StdDev: 0.5, Range: 1.0, CV: 0.067, Rating: "stable"},
		},
		MostStable:    "c1",
		MostSensitive: "c1",
		OverallCV:     0.067,
	}
	text := FormatText(r)
	if text == "" {
		t.Error("format text should not be empty")
	}
}

func TestUnscoredReflectionsSkipped(t *testing.T) {
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "lab1", Model: "m1", Reviewer: "c1", Scored: false, Final: 0},
			{Lab: "lab2", Model: "m1", Reviewer: "c1", Scored: true, Final: 0},
		},
	}
	r := Analyze(rr)
	if len(r.Checkers) != 0 {
		t.Errorf("expected 0 checkers (all skipped), got %d", len(r.Checkers))
	}
}

func TestOverallCV(t *testing.T) {
	rr := model.RunReport{
		Reflection: []model.ReflectionResult{
			{Lab: "l1", Model: "m1", Reviewer: "c1", PromptVariant: "v1", Scored: true, Final: 10},
			{Lab: "l1", Model: "m1", Reviewer: "c1", PromptVariant: "v2", Scored: true, Final: 10},
			{Lab: "l1", Model: "m1", Reviewer: "c2", PromptVariant: "v1", Scored: true, Final: 5},
			{Lab: "l1", Model: "m1", Reviewer: "c2", PromptVariant: "v2", Scored: true, Final: 5},
		},
	}
	r := Analyze(rr)
	if math.Abs(r.OverallCV) > 0.001 {
		t.Errorf("overall CV = %.4f, want ~0 (all stable)", r.OverallCV)
	}
}
