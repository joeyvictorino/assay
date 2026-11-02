package fleet

import (
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeRR(models []string, reflections []model.ReflectionResult, mrs []model.ModelResult) model.RunReport {
	return model.RunReport{
		Models:     mrs,
		Reflection: reflections,
	}
}

func TestAnalyzeFleetEmpty(t *testing.T) {
	r := AnalyzeFleet(model.RunReport{}, nil)
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(r.Models))
	}
	if r.BestModel != "" {
		t.Errorf("expected empty best model")
	}
}

func TestAnalyzeFleetSingleModel(t *testing.T) {
	rr := makeRR([]string{"gpt-4"}, []model.ReflectionResult{
		{Lab: "injection", Model: "gpt-4", Scored: true, Final: 8, Passed: true},
		{Lab: "leakage", Model: "gpt-4", Scored: true, Final: 6, Passed: false},
	}, []model.ModelResult{
		{Ref: model.ModelRef{Model: "gpt-4"}, Usage: model.Usage{InputTokens: 100, OutputTokens: 50}},
	})
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "gpt-4"}, DedupKey: "f1"},
		{Model: model.ModelRef{Model: "gpt-4"}, DedupKey: "f2"},
	}

	r := AnalyzeFleet(rr, findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].OverallScore != 7.0 {
		t.Errorf("expected score 7.0, got %.2f", r.Models[0].OverallScore)
	}
	if r.Models[0].FindingCount != 2 {
		t.Errorf("expected 2 findings, got %d", r.Models[0].FindingCount)
	}
	if r.Models[0].TokensUsed != 150 {
		t.Errorf("expected 150 tokens, got %d", r.Models[0].TokensUsed)
	}
	if r.BestModel != "gpt-4" || r.WorstModel != "gpt-4" {
		t.Error("single model should be both best and worst")
	}
}

func TestAnalyzeFleetMultiModel(t *testing.T) {
	rr := makeRR(nil, []model.ReflectionResult{
		{Lab: "injection", Model: "gpt-4", Scored: true, Final: 9, Passed: true},
		{Lab: "injection", Model: "claude", Scored: true, Final: 7, Passed: true},
	}, nil)
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "gpt-4"}, DedupKey: "f1"},
		{Model: model.ModelRef{Model: "claude"}, DedupKey: "f1"},
		{Model: model.ModelRef{Model: "gpt-4"}, DedupKey: "f2"},
	}

	r := AnalyzeFleet(rr, findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("expected gpt-4 as best, got %s", r.BestModel)
	}
	if len(r.Consensus) != 1 || r.Consensus[0] != "f1" {
		t.Errorf("expected consensus on f1, got %v", r.Consensus)
	}
	if len(r.Divergent) != 1 || r.Divergent[0] != "f2" {
		t.Errorf("expected divergent f2, got %v", r.Divergent)
	}
}

func TestAgreementRate(t *testing.T) {
	rr := makeRR(nil, []model.ReflectionResult{
		{Lab: "x", Model: "a", Scored: true, Final: 5, Passed: true},
		{Lab: "x", Model: "b", Scored: true, Final: 5, Passed: true},
	}, nil)
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "a"}, DedupKey: "f1"},
		{Model: model.ModelRef{Model: "b"}, DedupKey: "f1"},
		{Model: model.ModelRef{Model: "a"}, DedupKey: "f2"},
		{Model: model.ModelRef{Model: "b"}, DedupKey: "f2"},
	}
	r := AnalyzeFleet(rr, findings)
	if r.Agreement != 1.0 {
		t.Errorf("expected 100%% agreement, got %.2f", r.Agreement)
	}
}

func TestSortByFindings(t *testing.T) {
	r := FleetReport{
		Models: []ModelResult{
			{Model: "a", FindingCount: 2},
			{Model: "b", FindingCount: 5},
		},
	}
	r = SortBy(r, "findings")
	if r.Models[0].Model != "b" {
		t.Errorf("expected b first when sorting by findings, got %s", r.Models[0].Model)
	}
}

func TestSortByPassRate(t *testing.T) {
	r := FleetReport{
		Models: []ModelResult{
			{Model: "a", PassRate: 0.5},
			{Model: "b", PassRate: 0.9},
		},
	}
	r = SortBy(r, "pass-rate")
	if r.Models[0].Model != "b" {
		t.Errorf("expected b first when sorting by pass-rate, got %s", r.Models[0].Model)
	}
}

func TestFilterModel(t *testing.T) {
	r := FleetReport{
		Models: []ModelResult{
			{Model: "a"},
			{Model: "b"},
		},
	}
	r = FilterModel(r, "b")
	if len(r.Models) != 1 || r.Models[0].Model != "b" {
		t.Errorf("expected only model b, got %v", r.Models)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := FleetReport{}
	s := FormatText(r)
	if s != "No models found in run.\n" {
		t.Errorf("unexpected output: %s", s)
	}
}

func TestFormatTextWithModels(t *testing.T) {
	r := FleetReport{
		Models: []ModelResult{
			{Model: "gpt-4", OverallScore: 8.5, PassRate: 0.9, FindingCount: 3, TokensUsed: 1000},
		},
		BestModel:  "gpt-4",
		WorstModel: "gpt-4",
		Consensus:  []string{"f1"},
		Agreement:  1.0,
	}
	s := FormatText(r)
	if s == "" {
		t.Error("expected non-empty output")
	}
}
