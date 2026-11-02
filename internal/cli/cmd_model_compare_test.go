package cli

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestCompareModelsEmpty(t *testing.T) {
	r := compareModels(nil)
	if r.RunsAnalyzed != 0 {
		t.Errorf("expected 0 runs, got %d", r.RunsAnalyzed)
	}
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(r.Models))
	}
}

func TestCompareModelsSingleRun(t *testing.T) {
	reports := []model.RunReport{
		{
			Models: []model.ModelResult{
				{
					Ref:      model.ModelRef{Model: "gpt-4"},
					Findings: map[model.FindingState]int{"VALIDATED": 3, "THEORIZED": 1},
					P50MS:    500,
					Refusals: 1,
				},
				{
					Ref:      model.ModelRef{Model: "claude"},
					Findings: map[model.FindingState]int{"VALIDATED": 2},
					P50MS:    300,
					Refusals: 0,
				},
			},
		},
	}
	r := compareModels(reports)
	if r.RunsAnalyzed != 1 {
		t.Errorf("expected 1 run, got %d", r.RunsAnalyzed)
	}
	if len(r.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(r.Models))
	}
	// sorted by AvgFindings desc: gpt-4 (4) > claude (2)
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4 first, got %s", r.Models[0].Model)
	}
	if r.Models[0].TotalFindings != 4 {
		t.Errorf("expected 4 total findings for gpt-4, got %d", r.Models[0].TotalFindings)
	}
	if r.Models[1].TotalFindings != 2 {
		t.Errorf("expected 2 total findings for claude, got %d", r.Models[1].TotalFindings)
	}
}

func TestCompareModelsMultipleRuns(t *testing.T) {
	reports := []model.RunReport{
		{
			Models: []model.ModelResult{
				{Ref: model.ModelRef{Model: "m1"}, Findings: map[model.FindingState]int{"VALIDATED": 2}, P50MS: 100, Refusals: 0},
			},
		},
		{
			Models: []model.ModelResult{
				{Ref: model.ModelRef{Model: "m1"}, Findings: map[model.FindingState]int{"VALIDATED": 4}, P50MS: 200, Refusals: 1},
			},
		},
	}
	r := compareModels(reports)
	if r.RunsAnalyzed != 2 {
		t.Errorf("expected 2 runs, got %d", r.RunsAnalyzed)
	}
	if len(r.Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(r.Models))
	}
	m := r.Models[0]
	if m.RunCount != 2 {
		t.Errorf("expected run count 2, got %d", m.RunCount)
	}
	if m.TotalFindings != 6 {
		t.Errorf("expected 6 total findings, got %d", m.TotalFindings)
	}
	if m.AvgFindings != 3.0 {
		t.Errorf("expected avg 3.0, got %.1f", m.AvgFindings)
	}
	if m.AvgP50MS != 150 {
		t.Errorf("expected avg p50 150, got %d", m.AvgP50MS)
	}
	if m.Refusals != 1 {
		t.Errorf("expected 1 refusal, got %d", m.Refusals)
	}
}

func TestComputeOverlap(t *testing.T) {
	reports := []model.RunReport{
		{
			Models: []model.ModelResult{
				{Ref: model.ModelRef{Model: "a"}, Findings: map[model.FindingState]int{"VALIDATED": 3}},
				{Ref: model.ModelRef{Model: "b"}, Findings: map[model.FindingState]int{"VALIDATED": 5}},
			},
		},
	}
	ov := computeOverlap(reports, "a", "b")
	if ov.ModelA != "a" || ov.ModelB != "b" {
		t.Errorf("unexpected model names: %s, %s", ov.ModelA, ov.ModelB)
	}
	if ov.Shared != 3 {
		t.Errorf("expected shared=3, got %d", ov.Shared)
	}
	if ov.OnlyA != 0 {
		t.Errorf("expected onlyA=0, got %d", ov.OnlyA)
	}
	if ov.OnlyB != 2 {
		t.Errorf("expected onlyB=2, got %d", ov.OnlyB)
	}
	if ov.Agreement < 74 || ov.Agreement > 76 {
		t.Errorf("expected ~75%% agreement, got %.1f", ov.Agreement)
	}
}

func TestComputeOverlapZero(t *testing.T) {
	reports := []model.RunReport{{}}
	ov := computeOverlap(reports, "x", "y")
	if ov.Agreement != 0 {
		t.Errorf("expected 0 agreement, got %.1f", ov.Agreement)
	}
}

func TestFormatModelCompareText(t *testing.T) {
	r := modelCompareResult{
		RunsAnalyzed: 2,
		Models: []modelCompareEntry{
			{Model: "gpt-4", RunCount: 2, TotalFindings: 8, AvgFindings: 4.0, AvgP50MS: 400},
			{Model: "claude", RunCount: 2, TotalFindings: 6, AvgFindings: 3.0, AvgP50MS: 250},
		},
		Overlaps: []modelOverlap{
			{ModelA: "gpt-4", ModelB: "claude", Shared: 5, OnlyA: 3, OnlyB: 1, Agreement: 71.4},
		},
	}
	text := formatModelCompareText(r)
	if !strings.Contains(text, "Model Performance Comparison") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "Runs analyzed: 2") {
		t.Error("missing runs count")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("missing gpt-4")
	}
	if !strings.Contains(text, "claude") {
		t.Error("missing claude")
	}
	if !strings.Contains(text, "71.4%") {
		t.Error("missing agreement percentage")
	}
}
