package clarity

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func findings(summaries ...string) []model.Finding {
	var ff []model.Finding
	for _, s := range summaries {
		ff = append(ff, model.Finding{
			Summary:  s,
			Model:    model.ModelRef{Model: "test-model"},
			Severity: "medium",
		})
	}
	return ff
}

func TestClassifyAmbiguity(t *testing.T) {
	f := model.Finding{Summary: "It depends on many factors and could be either way"}
	d := classifyDimension(f)
	if d != DimAmbiguity {
		t.Errorf("expected DimAmbiguity, got %s", d)
	}
}

func TestClassifyJargon(t *testing.T) {
	f := model.Finding{Summary: "The paradigm shift requires leveraging synergy"}
	d := classifyDimension(f)
	if d != DimJargon {
		t.Errorf("expected DimJargon, got %s", d)
	}
}

func TestClassifyStructure(t *testing.T) {
	f := model.Finding{Summary: "Firstly we should consider the approach"}
	d := classifyDimension(f)
	if d != DimStructure {
		t.Errorf("expected DimStructure, got %s", d)
	}
}

func TestClassifySpecificity(t *testing.T) {
	f := model.Finding{Summary: "Specifically this applies to the case at hand"}
	d := classifyDimension(f)
	if d != DimSpecificity {
		t.Errorf("expected DimSpecificity, got %s", d)
	}
}

func TestClassifyGeneral(t *testing.T) {
	f := model.Finding{Summary: "A straightforward response about the topic"}
	d := classifyDimension(f)
	if d != DimGeneral {
		t.Errorf("expected DimGeneral, got %s", d)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected no models for nil findings")
	}
}

func TestAnalyzeSingle(t *testing.T) {
	ff := findings("it depends on many unclear factors")
	r := Analyze(ff)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].TotalFindings != 1 {
		t.Errorf("expected 1 finding, got %d", r.Models[0].TotalFindings)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	ff := []model.Finding{
		{Summary: "it depends", Model: model.ModelRef{Model: "gpt-4"}, Severity: "medium"},
		{Summary: "specifically this", Model: model.ModelRef{Model: "claude-3"}, Severity: "low"},
	}
	r := Analyze(ff)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0.95, "A"},
		{0.85, "B"},
		{0.75, "C"},
		{0.65, "D"},
		{0.45, "F"},
	}
	for _, tt := range tests {
		got := grade(tt.score)
		if got != tt.want {
			t.Errorf("grade(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestBestWorst(t *testing.T) {
	ff := []model.Finding{
		{Summary: "it depends could be perhaps arguably ambiguous", Model: model.ModelRef{Model: "ambiguous"}, Severity: "high"},
		{Summary: "specifically for example concretely", Model: model.ModelRef{Model: "clear"}, Severity: "low"},
	}
	r := Analyze(ff)
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst models to be set")
	}
}

func TestFormatText(t *testing.T) {
	ff := findings("it depends on context")
	r := Analyze(ff)
	text := FormatText(r)
	if text == "" {
		t.Fatal("empty text output")
	}
	if !strings.Contains(text, "Clarity Analysis") {
		t.Error("missing header")
	}
}

func TestFormatJSON(t *testing.T) {
	ff := findings("it depends on context")
	r := Analyze(ff)
	output := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestScoreRange(t *testing.T) {
	ff := findings("it depends", "leverage synergy", "specifically this")
	r := Analyze(ff)
	for _, m := range r.Models {
		if m.OverallScore < 0 || m.OverallScore > 1 {
			t.Errorf("score %f out of [0,1] range", m.OverallScore)
		}
	}
}

func TestClarityIndex(t *testing.T) {
	ff := findings("it depends on the context", "clear and direct")
	r := Analyze(ff)
	if r.Models[0].ClarityIndex < 0 || r.Models[0].ClarityIndex > 1 {
		t.Errorf("clarity index %f out of range", r.Models[0].ClarityIndex)
	}
}
