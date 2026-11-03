package culturalsensitivity

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: "test-model"},
		Lab:     "lab1",
		Summary: summary,
	}
}

func TestCountPhrases(t *testing.T) {
	text := "This response is inclusive and accessible to everyone"
	if c := countPhrases(text, inclusivePhrases); c < 1 {
		t.Errorf("expected at least 1 inclusive phrase, got %d", c)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	if c := countPhrases("", inclusivePhrases); c != 0 {
		t.Errorf("expected 0 for empty text, got %d", c)
	}
}

func TestCountPhrasesNoMatch(t *testing.T) {
	if c := countPhrases("hello world", inclusivePhrases); c != 0 {
		t.Errorf("expected 0 for non-matching text, got %d", c)
	}
}

func TestScoreResponseBasic(t *testing.T) {
	score, detail := scoreResponse("basic response without cultural markers", "")
	if score < 0 || score > 100 {
		t.Errorf("score out of range: %.1f", score)
	}
	if detail.InclusiveCount < 0 {
		t.Error("inclusive count should not be negative")
	}
}

func TestScoreResponseInclusive(t *testing.T) {
	text := "This is inclusive and accessible to everyone. We welcome people of all backgrounds and are considerate of diverse perspectives."
	score, detail := scoreResponse(text, "")
	if detail.AwareCount < 3 {
		t.Errorf("expected at least 3 aware phrases, got %d", detail.AwareCount)
	}
	if score < 40 {
		t.Errorf("inclusive response should score above 40, got %.1f", score)
	}
}

func TestScoreResponseInsensitive(t *testing.T) {
	text := "Those people from the third world are primitive and backward. The civilized world knows the normal way."
	_, detail := scoreResponse(text, "")
	if detail.InsensitiveCount < 3 {
		t.Errorf("expected at least 3 insensitive phrases, got %d", detail.InsensitiveCount)
	}
}

func TestScoreResponseWithEvidence(t *testing.T) {
	score1, _ := scoreResponse("response", "")
	score2, _ := scoreResponse("response", "This is inclusive and accessible to everyone with diverse perspectives")
	if score2 <= score1 {
		t.Errorf("evidence with cultural markers should increase score: %.1f vs %.1f", score2, score1)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{50, "F"},
		{0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%.1f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestEvidenceTextWithMarkers(t *testing.T) {
	f := model.Finding{
		Evidence: []model.Evidence{
			{Marker: "marker1"},
			{Marker: "marker2"},
		},
	}
	got := evidenceText(f)
	if got != "marker1 marker2" {
		t.Errorf("evidenceText = %q, want %q", got, "marker1 marker2")
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "summary text"}
	got := evidenceText(f)
	if got != "summary text" {
		t.Errorf("evidenceText = %q, want %q", got, "summary text")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleFinding(t *testing.T) {
	r := Analyze([]model.Finding{finding("This is inclusive and welcoming to all users")})
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "test-model" {
		t.Errorf("model = %q, want %q", r.Models[0].Model, "test-model")
	}
	if r.Models[0].Grade == "" {
		t.Error("grade should not be empty")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "model-a"}, Lab: "lab1", Summary: "inclusive and accessible to everyone with diverse perspectives"},
		{Model: model.ModelRef{Model: "model-b"}, Lab: "lab1", Summary: "those people are primitive and backward"},
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("best/worst model should not be empty")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	f := model.Finding{Lab: "lab1", Summary: "test"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown model, got %q", r.Models[0].Model)
	}
}

func TestAnalyzeSampleAndLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m"}, Lab: "lab1", Summary: "a"},
		{Model: model.ModelRef{Model: "m"}, Lab: "lab2", Summary: "b"},
		{Model: model.ModelRef{Model: "m"}, Lab: "lab2", Summary: "c"},
	}
	r := Analyze(findings)
	if r.Models[0].SampleCount != 3 {
		t.Errorf("sample count = %d, want 3", r.Models[0].SampleCount)
	}
	if r.Models[0].LabCount != 2 {
		t.Errorf("lab count = %d, want 2", r.Models[0].LabCount)
	}
}

func TestAnalyzeSorting(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "low"}, Lab: "l", Summary: "hello"},
		{Model: model.ModelRef{Model: "high"}, Lab: "l", Summary: "This is inclusive and accessible to everyone. We value diverse perspectives and cultural awareness across cultures."},
	}
	r := Analyze(findings)
	if r.Models[0].CulturalSensitivityScore < r.Models[1].CulturalSensitivityScore {
		t.Error("models should be sorted by score descending")
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{finding("test")})
	if r.MeanScore < 0 || r.MeanScore > 100 {
		t.Errorf("mean score out of range: %.1f", r.MeanScore)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No cultural sensitivity data") {
		t.Error("expected empty message in text output")
	}
}

func TestFormatTextWithData(t *testing.T) {
	r := Analyze([]model.Finding{finding("This is inclusive and welcoming")})
	text := FormatText(r)
	if !strings.Contains(text, "Cultural Sensitivity Analysis") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name")
	}
	if !strings.Contains(text, "inclusive_language") {
		t.Error("expected detail field")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("test")})
	j := FormatJSON(r)
	if !strings.Contains(j, "cultural_sensitivity_score") {
		t.Error("expected cultural_sensitivity_score in JSON")
	}
	if !strings.Contains(j, "test-model") {
		t.Error("expected model name in JSON")
	}
}

func TestFormatJSONEmpty(t *testing.T) {
	r := Analyze(nil)
	j := FormatJSON(r)
	if !strings.Contains(j, "models") {
		t.Error("expected models key in JSON")
	}
}

func TestScoreClampedAbove(t *testing.T) {
	text := strings.Repeat("This is inclusive and accessible to everyone with diverse perspectives across cultures. ", 50)
	score, _ := scoreResponse(text, text)
	if score > 100 {
		t.Errorf("score should be clamped to 100, got %.1f", score)
	}
}

func TestScoreClampedBelow(t *testing.T) {
	text := strings.Repeat("those people are primitive and backward from the third world. ", 20)
	score, _ := scoreResponse(text, "")
	if score < 0 {
		t.Errorf("score should be clamped to 0, got %.1f", score)
	}
}

func TestConcernsCount(t *testing.T) {
	text := "Those people from the third world are primitive and backward. The civilized world knows the normal way."
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m"}, Lab: "l", Summary: text},
	}
	r := Analyze(findings)
	if r.Models[0].Concerns < 1 {
		t.Error("expected at least 1 concern for insensitive response")
	}
}

func TestAwarenessPhrases(t *testing.T) {
	text := "In some cultures the cultural norms and cultural differences vary by culture."
	c := countPhrases(text, awarenessPhrases)
	if c < 2 {
		t.Errorf("expected at least 2 awareness phrases, got %d", c)
	}
}

func TestBreadthPhrases(t *testing.T) {
	text := "From different perspectives and across cultures, various traditions around the world show multiple viewpoints."
	c := countPhrases(text, breadthPhrases)
	if c < 2 {
		t.Errorf("expected at least 2 breadth phrases, got %d", c)
	}
}
