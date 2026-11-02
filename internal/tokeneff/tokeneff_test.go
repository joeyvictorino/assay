package tokeneff

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyzeBasic(t *testing.T) {
	rr := model.RunReport{
		RunID:        "eff-1",
		BudgetCapUSD: 10.0,
		SpentUSD:     2.5,
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "anthropic", Model: "claude-sonnet-4", Agent: "recon"},
				Usage: model.Usage{InputTokens: 50000, OutputTokens: 10000, CostUSD: 1.0},
				Findings: map[model.FindingState]int{
					model.StateValidated: 5,
					model.StateTheorized: 2,
					model.StateRefuted:   1,
				},
			},
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "probe"},
				Usage: model.Usage{InputTokens: 80000, OutputTokens: 20000, CostUSD: 1.5},
				Findings: map[model.FindingState]int{
					model.StateValidated: 3,
					model.StateTheorized: 1,
				},
			},
		},
	}

	r := Analyze(rr)
	if r.RunID != "eff-1" {
		t.Errorf("run_id = %q, want eff-1", r.RunID)
	}
	if r.Summary.TotalModels != 2 {
		t.Errorf("total_models = %d, want 2", r.Summary.TotalModels)
	}
	if r.Summary.TotalTokens != 160000 {
		t.Errorf("total_tokens = %d, want 160000", r.Summary.TotalTokens)
	}
}

func TestAnalyzeOrdering(t *testing.T) {
	rr := model.RunReport{
		RunID: "ord-1",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 100000, OutputTokens: 20000, CostUSD: 2.0},
				Findings: map[model.FindingState]int{
					model.StateValidated: 1,
				},
			},
			{
				Ref:   model.ModelRef{Provider: "anthropic", Model: "claude-sonnet-4", Agent: "probe"},
				Usage: model.Usage{InputTokens: 30000, OutputTokens: 5000, CostUSD: 0.5},
				Findings: map[model.FindingState]int{
					model.StateValidated: 5,
				},
			},
		},
	}

	r := Analyze(rr)
	if len(r.Models) != 2 {
		t.Fatalf("models = %d, want 2", len(r.Models))
	}
	if r.Models[0].EfficiencyScore < r.Models[1].EfficiencyScore {
		t.Error("models should be sorted by efficiency score descending")
	}
}

func TestAnalyzeNoFindings(t *testing.T) {
	rr := model.RunReport{
		RunID: "nofind-1",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 50000, OutputTokens: 10000, CostUSD: 1.0},
			},
		},
	}

	r := Analyze(rr)
	if r.Models[0].TokensPerFinding != 0 {
		t.Errorf("tokens_per_finding = %f, want 0 when no findings", r.Models[0].TokensPerFinding)
	}
}

func TestAnalyzeWithRefusals(t *testing.T) {
	rr := model.RunReport{
		RunID: "ref-1",
		Models: []model.ModelResult{
			{
				Ref:      model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage:    model.Usage{InputTokens: 50000, OutputTokens: 10000, CostUSD: 1.0},
				Refusals: 5,
			},
		},
	}

	r := Analyze(rr)
	if r.Models[0].EfficiencyScore != 0 {
		t.Errorf("efficiency score = %f, want 0 for model with only refusals", r.Models[0].EfficiencyScore)
	}
}

func TestBudgetUsedPct(t *testing.T) {
	rr := model.RunReport{
		RunID:        "budget-1",
		BudgetCapUSD: 5.0,
		SpentUSD:     2.5,
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "anthropic", Model: "claude-sonnet-4", Agent: "recon"},
				Usage: model.Usage{InputTokens: 50000, OutputTokens: 10000, CostUSD: 2.5},
				Findings: map[model.FindingState]int{
					model.StateValidated: 3,
				},
			},
		},
	}

	r := Analyze(rr)
	if r.Summary.BudgetUsedPct != 50.0 {
		t.Errorf("budget_used_pct = %f, want 50.0", r.Summary.BudgetUsedPct)
	}
}

func TestGradeScore(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{90, "A"},
		{80, "A"},
		{70, "B"},
		{55, "C"},
		{40, "D"},
		{20, "F"},
		{0, "F"},
	}
	for _, tt := range tests {
		if got := gradeScore(tt.score); got != tt.grade {
			t.Errorf("gradeScore(%f) = %q, want %q", tt.score, got, tt.grade)
		}
	}
}

func TestFormatText(t *testing.T) {
	r := Report{
		RunID:       "fmt-1",
		BestOverall: "claude-sonnet-4",
		BestValue:   "gpt-4o-mini",
		Models: []ModelEfficiency{
			{Agent: "recon", Model: "claude-sonnet-4", TotalTokens: 60000, FindingsTotal: 8, TokensPerFinding: 7500, EfficiencyScore: 75, Grade: "B"},
		},
		Summary: Summary{
			TotalModels:   1,
			TotalTokens:   60000,
			TotalCostUSD:  1.5,
			TotalFindings: 8,
			AvgEfficiency: 75,
			BudgetCapUSD:  10,
			BudgetUsedPct: 15,
		},
	}
	text := FormatText(r)
	if !strings.Contains(text, "Token Efficiency Analysis") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "claude-sonnet-4") {
		t.Error("missing model name")
	}
	if !strings.Contains(text, "recon") {
		t.Error("missing agent name")
	}
}

func TestCountFindings(t *testing.T) {
	m := map[model.FindingState]int{
		model.StateValidated: 3,
		model.StateTheorized: 2,
		model.StateRefuted:   1,
	}
	if got := countFindings(m); got != 6 {
		t.Errorf("countFindings = %d, want 6", got)
	}
}

func TestCountFindingsNil(t *testing.T) {
	if got := countFindings(nil); got != 0 {
		t.Errorf("countFindings(nil) = %d, want 0", got)
	}
}

func TestEmptyRunReport(t *testing.T) {
	r := Analyze(model.RunReport{RunID: "empty"})
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(r.Models))
	}
	if r.Summary.TotalModels != 0 {
		t.Errorf("total_models = %d, want 0", r.Summary.TotalModels)
	}
}
