package hypothetical

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(mdl, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mdl},
		Lab:     "lab1",
		Summary: summary,
	}
}

func TestCountPhrases(t *testing.T) {
	text := "What if we imagine if the scenario changes? Suppose that constraints apply."
	n := countPhrases(text, scenarioPhrases)
	if n < 2 {
		t.Errorf("expected at least 2 scenario phrases, got %d", n)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	if n := countPhrases("", scenarioPhrases); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	n := countPhrases("WHAT IF we IMAGINE IF things change", scenarioPhrases)
	if n < 2 {
		t.Errorf("expected at least 2, got %d", n)
	}
}

func TestScoreResponseBasic(t *testing.T) {
	text := "What if we consider a scenario where suppose that the constraints change? " +
		"Therefore it follows that the necessary conclusion is consistent with the premise. " +
		"Extrapolating from this, one possibility is the logical consequence."
	score, detail := scoreResponse(text, "")
	if score < 40 {
		t.Errorf("expected score >= 40 for hypothetical-rich text, got %.1f", score)
	}
	if detail.HypotheticalCount < 5 {
		t.Errorf("expected at least 5 hypothetical phrases, got %d", detail.HypotheticalCount)
	}
}

func TestScoreResponseWeakPenalty(t *testing.T) {
	text := "I'm not sure what would happen. It's hard to say. Anything could happen. " +
		"Impossible to predict. Who knows what the outcome might be."
	score, detail := scoreResponse(text, "")
	if detail.WeakReasoningCount < 3 {
		t.Errorf("expected at least 3 weak reasoning phrases, got %d", detail.WeakReasoningCount)
	}
	if score > 50 {
		t.Errorf("expected penalized score <= 50, got %.1f", score)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	huge := strings.Repeat("what if imagine if suppose that consider a scenario ", 200)
	score, _ := scoreResponse(huge, "")
	if score > 100 {
		t.Errorf("score should not exceed 100, got %.1f", score)
	}
	if score < 0 {
		t.Errorf("score should not be negative, got %.1f", score)
	}

	bad := strings.Repeat("i'm not sure what would happen anything could happen impossible to predict ", 100)
	score2, _ := scoreResponse(bad, "")
	if score2 < 0 {
		t.Errorf("score should not be negative, got %.1f", score2)
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, detail := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Errorf("empty score out of range: %.1f", score)
	}
	if detail.HypotheticalCount != 0 {
		t.Errorf("expected 0 hypothetical phrases for empty, got %d", detail.HypotheticalCount)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {90, "A"},
		{85, "B"}, {80, "B"},
		{75, "C"}, {70, "C"},
		{65, "D"}, {60, "D"},
		{55, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		if g := scoreToGrade(tt.score); g != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, g, tt.grade)
		}
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected empty models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "What if we consider a scenario where therefore it follows that"),
		finding("gpt-4", "Suppose that extrapolating from this the logical consequence is"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("expected best=gpt-4, got %s", r.BestModel)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("modelA", "What if we imagine if suppose that therefore consequently it follows that"),
		finding("modelB", "i'm not sure what would happen anything could happen who knows"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	f := model.Finding{Lab: "lab1", Summary: "what if we consider"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	out := FormatText(Report{})
	if !strings.Contains(out, "No hypothetical reasoning data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{finding("test", "what if suppose that therefore")})
	out := FormatText(r)
	if !strings.Contains(out, "Hypothetical Reasoning Analysis") {
		t.Error("expected header")
	}
	if !strings.Contains(out, "test") {
		t.Error("expected model name")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("test", "what if therefore")})
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("expected 1 model in JSON, got %d", len(parsed.Models))
	}
}
