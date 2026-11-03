package analogical

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: m},
		Summary: summary,
		Lab:     "test-lab",
	}
}

func TestCountPhrases(t *testing.T) {
	text := "is like something similar to another thing"
	got := countPhrases(text, analogyPhrases)
	if got < 2 {
		t.Errorf("expected >=2 analogy phrases, got %d", got)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	got := countPhrases("", analogyPhrases)
	if got != 0 {
		t.Errorf("expected 0 phrases for empty text, got %d", got)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "IS LIKE and SIMILAR TO big ideas"
	got := countPhrases(text, analogyPhrases)
	if got < 2 {
		t.Errorf("expected >=2 analogy phrases (case-insensitive), got %d", got)
	}
}

func TestScoreResponseHighAnalogy(t *testing.T) {
	text := "is like similar to analogous to think of it as just as comparable to maps to corresponds to the underlying principle applying this to"
	score, detail := scoreResponse(text, "")
	if score < 50 {
		t.Errorf("expected high-analogy score >=50, got %.1f", score)
	}
	if detail.AnalogyCount < 5 {
		t.Errorf("expected analogy count >=5, got %d", detail.AnalogyCount)
	}
}

func TestScoreResponseLowAnalogy(t *testing.T) {
	text := "the cat sat on the mat"
	score, detail := scoreResponse(text, "")
	if detail.AnalogyCount != 0 {
		t.Errorf("expected 0 analogy count, got %d", detail.AnalogyCount)
	}
	if score > 70 {
		t.Errorf("expected moderate score for neutral text, got %.1f", score)
	}
}

func TestScoreResponseWeak(t *testing.T) {
	text := "sort of like kind of similar loosely related vaguely resembles a rough comparison stretching the analogy"
	score, detail := scoreResponse(text, "")
	if detail.WeakAnalogyCount < 4 {
		t.Errorf("expected weak count >=4, got %d", detail.WeakAnalogyCount)
	}
	if score > 50 {
		t.Errorf("expected low score for weak analogies, got %.1f", score)
	}
}

func TestScoreResponseForced(t *testing.T) {
	text := "forced comparison false equivalence apples and oranges the analogy fails not really comparable"
	_, detail := scoreResponse(text, "")
	if detail.ForcedCount < 4 {
		t.Errorf("expected forced count >=4, got %d", detail.ForcedCount)
	}
}

func TestScoreResponseBounds(t *testing.T) {
	score1, _ := scoreResponse("", "")
	if score1 < 0 || score1 > 100 {
		t.Errorf("score out of bounds: %.1f", score1)
	}

	longText := strings.Repeat("is like similar to analogous to maps to ", 100)
	score2, _ := scoreResponse(longText, "")
	if score2 > 100 {
		t.Errorf("score exceeds 100: %.1f", score2)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {90, "A"}, {85, "B"}, {80, "B"},
		{75, "C"}, {70, "C"}, {65, "D"}, {60, "D"},
		{55, "F"}, {0, "F"},
	}
	for _, tc := range tests {
		got := scoreToGrade(tc.score)
		if got != tc.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tc.score, got, tc.grade)
		}
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback summary",
		Evidence: []model.Evidence{
			{Marker: "marker one"},
			{Marker: "marker two"},
		},
	}
	got := evidenceText(f)
	if got != "marker one marker two" {
		t.Errorf("unexpected evidence text: %s", got)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "my summary"}
	got := evidenceText(f)
	if got != "my summary" {
		t.Errorf("expected fallback to summary, got: %s", got)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected no models for nil input")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "is like a bridge similar to how water flows"),
		finding("gpt-4", "analogous to the way maps to the equivalent of"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("expected best model gpt-4, got %s", r.BestModel)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("model-a", "is like similar to analogous to maps to corresponds to the underlying principle"),
		finding("model-b", "the answer is 42"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Errorf("best and worst should differ with different quality responses")
	}
	if r.Models[0].AnalogScore < r.Models[1].AnalogScore {
		t.Errorf("models should be sorted descending by score")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "is like a mirror that parallels reality", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.BestModel != "unknown" {
		t.Errorf("expected unknown model, got %s", r.BestModel)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab2"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "sort of like kind of similar loosely related vaguely resembles a rough comparison stretching the analogy"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns < 1 {
		t.Errorf("expected concerns for weak analogy response")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Report{}
	text := FormatText(r)
	if !strings.Contains(text, "No analogical reasoning data") {
		t.Errorf("expected empty message in text output")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "is like a bridge similar to how maps to the equivalent of"),
	})
	text := FormatText(r)
	if !strings.Contains(text, "test-model") {
		t.Errorf("expected model name in text output")
	}
	if !strings.Contains(text, "analogy_fit") {
		t.Errorf("expected detail fields in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "is like and similar to ideas"),
	})
	j := FormatJSON(r)
	if !strings.Contains(j, "analogical_score") {
		t.Errorf("expected analogical_score in JSON output")
	}
	if !strings.Contains(j, "analogy_fit") {
		t.Errorf("expected analogy_fit in JSON output")
	}
}

func TestMeanScoreCalculation(t *testing.T) {
	findings := []model.Finding{
		finding("a", "is like similar to analogous to"),
		finding("b", "the cat sat on the mat"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Errorf("expected positive mean score, got %.1f", r.MeanScore)
	}
}

func TestMappingDetection(t *testing.T) {
	text := "corresponds to and maps to the equivalent of plays the role of serves as"
	_, detail := scoreResponse(text, "")
	if detail.MappingPrecision < 0.3 {
		t.Errorf("expected higher mapping precision score, got %.2f", detail.MappingPrecision)
	}
}

func TestTransferDetection(t *testing.T) {
	text := "applying this to another domain transferring this idea by extension this principle applies the same logic holds"
	_, detail := scoreResponse(text, "")
	if detail.TransferQuality < 0.3 {
		t.Errorf("expected higher transfer quality score, got %.2f", detail.TransferQuality)
	}
}
