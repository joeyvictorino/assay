package compositionality

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
	text := "combining multiple concepts and integrating ideas"
	got := countPhrases(text, integrationPhrases)
	if got < 2 {
		t.Errorf("expected >=2 integration phrases, got %d", got)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	got := countPhrases("", integrationPhrases)
	if got != 0 {
		t.Errorf("expected 0 phrases for empty text, got %d", got)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "COMBINING and INTEGRATING big ideas"
	got := countPhrases(text, integrationPhrases)
	if got < 2 {
		t.Errorf("expected >=2 integration phrases (case-insensitive), got %d", got)
	}
}

func TestScoreResponseHighComposite(t *testing.T) {
	text := "combining integrating merging bringing together synthesizing unifying building on this from a higher perspective therefore consequently as a result"
	score, detail := scoreResponse(text, "")
	if score < 50 {
		t.Errorf("expected high-composite score >=50, got %.1f", score)
	}
	if detail.CompositeCount < 5 {
		t.Errorf("expected composite count >=5, got %d", detail.CompositeCount)
	}
}

func TestScoreResponseLowComposite(t *testing.T) {
	text := "the cat sat on the mat"
	score, detail := scoreResponse(text, "")
	if detail.CompositeCount != 0 {
		t.Errorf("expected 0 composite count, got %d", detail.CompositeCount)
	}
	if score > 70 {
		t.Errorf("expected moderate score for neutral text, got %.1f", score)
	}
}

func TestScoreResponseFragmented(t *testing.T) {
	text := "separately in isolation unrelated to independently without connection shifting gears"
	score, detail := scoreResponse(text, "")
	if detail.FragmentedCount < 4 {
		t.Errorf("expected fragmented count >=4, got %d", detail.FragmentedCount)
	}
	if score > 50 {
		t.Errorf("expected low score for fragmented text, got %.1f", score)
	}
}

func TestScoreResponseShallow(t *testing.T) {
	text := "simply put basically nothing more than merely at face value superficially"
	_, detail := scoreResponse(text, "")
	if detail.ShallowCount < 4 {
		t.Errorf("expected shallow count >=4, got %d", detail.ShallowCount)
	}
}

func TestScoreResponseBounds(t *testing.T) {
	score1, _ := scoreResponse("", "")
	if score1 < 0 || score1 > 100 {
		t.Errorf("score out of bounds: %.1f", score1)
	}

	longText := strings.Repeat("combining integrating merging synthesizing ", 100)
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
		finding("gpt-4", "combining concepts and integrating ideas from different fields"),
		finding("gpt-4", "therefore this leads to a unified understanding building on this"),
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
		finding("model-a", "combining integrating synthesizing merging bringing together unifying"),
		finding("model-b", "the answer is 42"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Errorf("best and worst should differ with different quality responses")
	}
	if r.Models[0].ComposScore < r.Models[1].ComposScore {
		t.Errorf("models should be sorted descending by score")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "combining ideas together with merging", Lab: "lab1"},
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
		finding("m1", "separately in isolation unrelated to independently without connection disconnected shifting gears"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns < 1 {
		t.Errorf("expected concerns for fragmented response")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Report{}
	text := FormatText(r)
	if !strings.Contains(text, "No compositionality data") {
		t.Errorf("expected empty message in text output")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "combining ideas therefore this leads to synthesis"),
	})
	text := FormatText(r)
	if !strings.Contains(text, "test-model") {
		t.Errorf("expected model name in text output")
	}
	if !strings.Contains(text, "concept_integration") {
		t.Errorf("expected detail fields in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "combining and synthesizing ideas"),
	})
	j := FormatJSON(r)
	if !strings.Contains(j, "compositionality_score") {
		t.Errorf("expected compositionality_score in JSON output")
	}
	if !strings.Contains(j, "concept_integration") {
		t.Errorf("expected concept_integration in JSON output")
	}
}

func TestMeanScoreCalculation(t *testing.T) {
	findings := []model.Finding{
		finding("a", "combining integrating synthesizing"),
		finding("b", "the cat sat on the mat"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Errorf("expected positive mean score, got %.1f", r.MeanScore)
	}
}

func TestCrossDomainDetection(t *testing.T) {
	text := "cross-disciplinary approach drawing from interdisciplinary research across fields"
	_, detail := scoreResponse(text, "")
	if detail.CrossDomainLink < 0.3 {
		t.Errorf("expected higher cross-domain link score, got %.2f", detail.CrossDomainLink)
	}
}
