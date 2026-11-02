package promptdiversity

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
	if r.OverallEntropy != 0 {
		t.Fatalf("expected 0 entropy, got %f", r.OverallEntropy)
	}
}

func TestAnalyzeSinglePrompt(t *testing.T) {
	findings := []model.Finding{
		{PromptHash: "abc", Model: model.ModelRef{Model: "m1"}},
		{PromptHash: "abc", Model: model.ModelRef{Model: "m1"}},
		{PromptHash: "abc", Model: model.ModelRef{Model: "m1"}},
	}
	r := Analyze(findings)
	if r.Models[0].UniquePrompts != 1 {
		t.Errorf("expected 1 unique prompt, got %d", r.Models[0].UniquePrompts)
	}
	if r.OverallEntropy != 0 {
		t.Errorf("expected 0 entropy for single prompt, got %f", r.OverallEntropy)
	}
}

func TestAnalyzeMaxDiversity(t *testing.T) {
	findings := []model.Finding{
		{PromptHash: "a", Model: model.ModelRef{Model: "m1"}},
		{PromptHash: "b", Model: model.ModelRef{Model: "m1"}},
		{PromptHash: "c", Model: model.ModelRef{Model: "m1"}},
		{PromptHash: "d", Model: model.ModelRef{Model: "m1"}},
	}
	r := Analyze(findings)
	if r.Models[0].UniquePrompts != 4 {
		t.Errorf("expected 4 unique prompts, got %d", r.Models[0].UniquePrompts)
	}
	if r.Models[0].Ratio != 1.0 {
		t.Errorf("expected 1.0 ratio, got %f", r.Models[0].Ratio)
	}
	expectedEntropy := 2.0
	if math.Abs(r.OverallEntropy-expectedEntropy) > 0.01 {
		t.Errorf("expected ~2.0 entropy for 4 uniform prompts, got %f", r.OverallEntropy)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	findings := []model.Finding{
		{PromptHash: "a", Model: model.ModelRef{Model: "m1"}},
		{PromptHash: "b", Model: model.ModelRef{Model: "m1"}},
		{PromptHash: "a", Model: model.ModelRef{Model: "m2"}},
		{PromptHash: "a", Model: model.ModelRef{Model: "m2"}},
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	m1 := r.Models[0]
	m2 := r.Models[1]
	if m1.Model == "m2" {
		m1, m2 = m2, m1
	}
	if m1.UniquePrompts != 2 {
		t.Errorf("m1: expected 2 unique prompts, got %d", m1.UniquePrompts)
	}
	if m2.UniquePrompts != 1 {
		t.Errorf("m2: expected 1 unique prompt, got %d", m2.UniquePrompts)
	}
}

func TestShannon(t *testing.T) {
	counts := map[string]int{"a": 1, "b": 1}
	h := shannon(counts, 2)
	if math.Abs(h-1.0) > 0.001 {
		t.Errorf("expected 1.0 bit, got %f", h)
	}

	h2 := shannon(nil, 0)
	if h2 != 0 {
		t.Errorf("expected 0 for empty, got %f", h2)
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze([]model.Finding{
		{PromptHash: "x", Model: model.ModelRef{Model: "alpha"}},
	})
	text := FormatText(r)
	if !strings.Contains(text, "Prompt Template Diversity") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "alpha") {
		t.Error("missing model")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Report{Models: []ModelDiversity{{Model: "m1", Entropy: 1.5}}, OverallEntropy: 1.5}
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}
