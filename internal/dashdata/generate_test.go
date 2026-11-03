package dashdata

import (
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeRun(score float64, reflections []model.ReflectionResult) model.RunReport {
	return model.RunReport{
		Started:       time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
		Finished:      time.Date(2025, 1, 1, 10, 5, 0, 0, time.UTC),
		WeightedScore: score,
		Models:        []model.ModelResult{{Ref: model.ModelRef{Provider: "test", Model: "m1"}}},
		Reflection:    reflections,
	}
}

func TestGenerateEmpty(t *testing.T) {
	rr := model.RunReport{}
	d := Generate(rr, nil)
	if d.GeneratedAt == "" {
		t.Error("generated_at should be set")
	}
	if d.RunSummary.TotalModels != 0 {
		t.Errorf("total models = %d, want 0", d.RunSummary.TotalModels)
	}
}

func TestRunSummaryDuration(t *testing.T) {
	rr := makeRun(5.0, nil)
	d := Generate(rr, nil)
	if d.RunSummary.Duration != "5m0s" {
		t.Errorf("duration = %s, want 5m0s", d.RunSummary.Duration)
	}
	if d.RunSummary.TotalModels != 1 {
		t.Errorf("total models = %d, want 1", d.RunSummary.TotalModels)
	}
}

func TestModelScoresRanked(t *testing.T) {
	refs := []model.ReflectionResult{
		{Lab: "l1", Model: "fast", Reviewer: "c1", Scored: true, Final: 9},
		{Lab: "l1", Model: "slow", Reviewer: "c1", Scored: true, Final: 3},
	}
	rr := makeRun(6.0, refs)
	d := Generate(rr, nil)
	if len(d.ModelScores) != 2 {
		t.Fatalf("model scores = %d, want 2", len(d.ModelScores))
	}
	if d.ModelScores[0].Model != "fast" {
		t.Errorf("rank 1 model = %s, want fast", d.ModelScores[0].Model)
	}
	if d.ModelScores[0].Rank != 1 {
		t.Errorf("rank = %d, want 1", d.ModelScores[0].Rank)
	}
}

func TestModelScoresDelta(t *testing.T) {
	refs := []model.ReflectionResult{
		{Lab: "l1", Model: "m1", Reviewer: "c1", Scored: true, Final: 8},
	}
	rr := makeRun(8.0, refs)
	prevRefs := []model.ReflectionResult{
		{Lab: "l1", Model: "m1", Reviewer: "c1", Scored: true, Final: 5},
	}
	prev := makeRun(5.0, prevRefs)
	d := Generate(rr, &prev)
	if len(d.ModelScores) != 1 {
		t.Fatalf("model scores = %d, want 1", len(d.ModelScores))
	}
	if d.ModelScores[0].Delta != 3.0 {
		t.Errorf("delta = %.2f, want 3.0", d.ModelScores[0].Delta)
	}
}

func TestCheckerCards(t *testing.T) {
	refs := []model.ReflectionResult{
		{Lab: "l1", Model: "m1", Reviewer: "c1", Passed: true, Validated: 2},
		{Lab: "l2", Model: "m1", Reviewer: "c1", Passed: false, Validated: 1},
	}
	rr := makeRun(5.0, refs)
	d := Generate(rr, nil)
	if len(d.CheckerResults) != 1 {
		t.Fatalf("checker cards = %d, want 1", len(d.CheckerResults))
	}
	if d.CheckerResults[0].PassRate != 0.5 {
		t.Errorf("pass rate = %.2f, want 0.5", d.CheckerResults[0].PassRate)
	}
	if d.CheckerResults[0].Findings != 3 {
		t.Errorf("findings = %d, want 3", d.CheckerResults[0].Findings)
	}
}

func TestRiskGaugeLevels(t *testing.T) {
	tests := []struct {
		score float64
		level string
	}{
		{1.0, "low"},
		{3.0, "medium"},
		{6.0, "high"},
		{8.0, "critical"},
	}
	for _, tt := range tests {
		rr := makeRun(tt.score, nil)
		d := Generate(rr, nil)
		if d.RiskGauge.Level != tt.level {
			t.Errorf("score %.1f: level = %s, want %s", tt.score, d.RiskGauge.Level, tt.level)
		}
	}
}

func TestRiskGaugeTrend(t *testing.T) {
	rr := makeRun(8.0, nil)
	prev := makeRun(2.0, nil)
	d := Generate(rr, &prev)
	if d.RiskGauge.Trend != "degrading" {
		t.Errorf("trend = %s, want degrading", d.RiskGauge.Trend)
	}

	rr2 := makeRun(2.0, nil)
	prev2 := makeRun(8.0, nil)
	d2 := Generate(rr2, &prev2)
	if d2.RiskGauge.Trend != "improving" {
		t.Errorf("trend = %s, want improving", d2.RiskGauge.Trend)
	}
}

func TestTopFindingsCapped(t *testing.T) {
	var refs []model.ReflectionResult
	for i := 0; i < 20; i++ {
		refs = append(refs, model.ReflectionResult{
			Lab: "lab", Model: "m1", Reviewer: "c1", Validated: 1, Final: 3,
		})
	}
	rr := makeRun(5.0, refs)
	d := Generate(rr, nil)
	if len(d.TopFindings) > 10 {
		t.Errorf("top findings = %d, want <= 10", len(d.TopFindings))
	}
}

func TestNoPrev(t *testing.T) {
	rr := makeRun(5.0, nil)
	d := Generate(rr, nil)
	if d.RiskGauge.Trend != "stable" {
		t.Errorf("trend = %s, want stable (no prev)", d.RiskGauge.Trend)
	}
}
