package costbreak

import (
	"math"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyzeCostEmpty(t *testing.T) {
	rr := model.RunReport{}
	r := AnalyzeCost(rr)
	if len(r.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(r.Entries))
	}
	if r.TotalCostUSD != 0 {
		t.Errorf("expected 0 cost, got %.4f", r.TotalCostUSD)
	}
}

func TestAnalyzeCostSingleModel(t *testing.T) {
	rr := model.RunReport{
		RunID: "test-run",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{InputTokens: 1000000, OutputTokens: 500000, CostUSD: 0},
			},
		},
	}
	r := AnalyzeCost(rr)
	if len(r.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(r.Entries))
	}
	e := r.Entries[0]
	if e.Checker != "recon" {
		t.Errorf("checker = %q, want recon", e.Checker)
	}
	if e.Model != "gpt-4o" {
		t.Errorf("model = %q, want gpt-4o", e.Model)
	}
	// gpt-4o: $2.50/MTok input + $10.00/MTok output = $2.50 + $5.00 = $7.50
	expected := 2.50 + 5.00
	if math.Abs(e.EstCostUSD-expected) > 0.01 {
		t.Errorf("cost = %.4f, want ~%.2f", e.EstCostUSD, expected)
	}
}

func TestAnalyzeCostUsesExistingCost(t *testing.T) {
	rr := model.RunReport{
		RunID: "test-run",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "anthropic", Model: "claude-3-5-sonnet-20241022", Agent: "probe"},
				Usage: model.Usage{InputTokens: 100000, OutputTokens: 50000, CostUSD: 1.23},
			},
		},
	}
	r := AnalyzeCost(rr)
	if len(r.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(r.Entries))
	}
	if r.Entries[0].EstCostUSD != 1.23 {
		t.Errorf("cost = %.4f, want 1.23", r.Entries[0].EstCostUSD)
	}
}

func TestAnalyzeCostGrouping(t *testing.T) {
	rr := model.RunReport{
		RunID: "test-run",
		Models: []model.ModelResult{
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "recon"},
				Usage: model.Usage{CostUSD: 2.00},
			},
			{
				Ref:   model.ModelRef{Provider: "openai", Model: "gpt-4o", Agent: "probe"},
				Usage: model.Usage{CostUSD: 3.00},
			},
			{
				Ref:   model.ModelRef{Provider: "anthropic", Model: "claude-3-5-sonnet", Agent: "recon"},
				Usage: model.Usage{CostUSD: 1.50},
			},
		},
	}
	r := AnalyzeCost(rr)
	if math.Abs(r.TotalCostUSD-6.50) > 0.01 {
		t.Errorf("total cost = %.4f, want 6.50", r.TotalCostUSD)
	}
	if math.Abs(r.ByChecker["recon"]-3.50) > 0.01 {
		t.Errorf("recon cost = %.4f, want 3.50", r.ByChecker["recon"])
	}
	if math.Abs(r.ByModel["gpt-4o"]-5.00) > 0.01 {
		t.Errorf("gpt-4o cost = %.4f, want 5.00", r.ByModel["gpt-4o"])
	}
}

func TestLookupPricingKnown(t *testing.T) {
	p := lookupPricing("gpt-4o")
	if p.inputPerMTok != 2.50 {
		t.Errorf("gpt-4o input pricing = %.2f, want 2.50", p.inputPerMTok)
	}
}

func TestLookupPricingUnknown(t *testing.T) {
	p := lookupPricing("unknown-model-xyz")
	if p.inputPerMTok != 5.00 {
		t.Errorf("unknown model input pricing = %.2f, want 5.00 (default)", p.inputPerMTok)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := CostReport{}
	text := FormatText(r, "")
	if text == "" {
		t.Error("expected non-empty text for empty report")
	}
}

func TestFormatTextGroupByChecker(t *testing.T) {
	r := CostReport{
		Entries:     []CostEntry{{Checker: "recon", Model: "gpt-4o", EstCostUSD: 1.50}},
		TotalTokens: 100000,
		TotalCostUSD: 1.50,
		ByChecker:   map[string]float64{"recon": 1.50},
		ByModel:     map[string]float64{"gpt-4o": 1.50},
	}
	text := FormatText(r, "checker")
	if text == "" {
		t.Error("expected non-empty text")
	}
}

func TestFormatTextGroupByModel(t *testing.T) {
	r := CostReport{
		Entries:     []CostEntry{{Checker: "recon", Model: "gpt-4o", EstCostUSD: 1.50}},
		TotalTokens: 100000,
		TotalCostUSD: 1.50,
		ByChecker:   map[string]float64{"recon": 1.50},
		ByModel:     map[string]float64{"gpt-4o": 1.50},
	}
	text := FormatText(r, "model")
	if text == "" {
		t.Error("expected non-empty text")
	}
}
