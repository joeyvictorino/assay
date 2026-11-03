package modelcost

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(id, mdl, provider string) model.Finding {
	return model.Finding{
		ID:      id,
		Model:   model.ModelRef{Provider: provider, Model: mdl},
		Summary: "test finding " + id,
		Class:   "injection",
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.Summary.TotalModels != 0 {
		t.Fatalf("expected 0 models, got %d", r.Summary.TotalModels)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "gpt-4", "openai"),
		makeFinding("f2", "gpt-4", "openai"),
	}
	r := Analyze(findings)
	if r.Summary.TotalModels != 1 {
		t.Fatalf("expected 1 model, got %d", r.Summary.TotalModels)
	}
	if r.Summary.TotalFindings != 2 {
		t.Fatalf("expected 2 findings, got %d", r.Summary.TotalFindings)
	}
	if r.Models[0].FindingCount != 2 {
		t.Fatalf("expected 2 findings for model, got %d", r.Models[0].FindingCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "gpt-4", "openai"),
		makeFinding("f2", "gpt-4o-mini", "openai"),
		makeFinding("f3", "claude-3-haiku", "anthropic"),
	}
	r := Analyze(findings)
	if r.Summary.TotalModels != 3 {
		t.Fatalf("expected 3 models, got %d", r.Summary.TotalModels)
	}
}

func TestBestValueIsCheapest(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "gpt-4", "openai"),
		makeFinding("f2", "gpt-4o-mini", "openai"),
	}
	r := Analyze(findings)
	gpt4Cost := 0.0
	miniCost := 0.0
	for _, m := range r.Models {
		if m.Model == "gpt-4" {
			gpt4Cost = m.CostPerFinding
		}
		if m.Model == "gpt-4o-mini" {
			miniCost = m.CostPerFinding
		}
	}
	if miniCost >= gpt4Cost {
		t.Fatalf("gpt-4o-mini ($%.4f) should be cheaper than gpt-4 ($%.4f)", miniCost, gpt4Cost)
	}
}

func TestCostPerFindingPositive(t *testing.T) {
	findings := []model.Finding{makeFinding("f1", "claude-3-opus", "anthropic")}
	r := Analyze(findings)
	if r.Models[0].CostPerFinding <= 0 {
		t.Fatal("expected positive cost per finding")
	}
}

func TestFormatTextContainsHeader(t *testing.T) {
	r := Analyze(nil)
	out := FormatText(r)
	if !strings.Contains(out, "Model Cost Analysis") {
		t.Fatal("expected header")
	}
}

func TestFormatJSONContainsKey(t *testing.T) {
	findings := []model.Finding{makeFinding("f1", "gpt-4", "openai")}
	r := Analyze(findings)
	out := FormatJSON(r)
	if !strings.Contains(out, `"total_estimated_cost_usd"`) {
		t.Fatal("expected cost key in JSON")
	}
}

func TestUnknownModelGetsDefaultPrice(t *testing.T) {
	findings := []model.Finding{makeFinding("f1", "custom-model-v1", "custom")}
	r := Analyze(findings)
	if r.Models[0].EstCost <= 0 {
		t.Fatal("expected positive cost for unknown model")
	}
}

func TestEstimateTokensMinimum(t *testing.T) {
	f := model.Finding{Summary: "x"}
	tokens := estimateTokens(f)
	if tokens < 500 {
		t.Fatalf("expected minimum 500 tokens, got %d", tokens)
	}
}
