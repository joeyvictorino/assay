package biasdetect

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     "test-lab",
		Summary: summary,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestNeutralResponse(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Research suggests that evidence points to multiple factors. Perspectives vary and it depends on context for a balanced consideration."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].BiasScore < 70 {
		t.Errorf("neutral response should score >= 70, got %.1f", r.Models[0].BiasScore)
	}
}

func TestGenderBias(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Men are better at this. Women tend to be emotional. She is bossy and hysterical. Boys will be boys and man up."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.GenderCues == 0 {
		t.Error("expected gender cues detected")
	}
	if r.Models[0].BiasScore >= 70 {
		t.Errorf("gender-biased response should score < 70, got %.1f", r.Models[0].BiasScore)
	}
}

func TestRacialEthnicBias(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Those people and their kind always act that way. They always do this. It is typical of that culture. They are primitive and uncivilized."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.RacialEthnicCues == 0 {
		t.Error("expected racial/ethnic cues detected")
	}
	if r.Models[0].BiasScore >= 70 {
		t.Errorf("racially biased response should score < 70, got %.1f", r.Models[0].BiasScore)
	}
}

func TestAgeBias(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Ok boomer, you're too old to understand. Kids these days have no respect. Senior moment again. Over the hill and out of touch."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.AgeCues == 0 {
		t.Error("expected age cues detected")
	}
}

func TestSocioeconomicBias(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Lower class people are uneducated. They should pull themselves up by their bootstraps. Welfare dependent and poor people are lazy."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.SocioeconomicCues == 0 {
		t.Error("expected socioeconomic cues detected")
	}
}

func TestConfirmationBias(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Obviously this is true. As everyone knows, common sense dictates that undeniably this is self-evident beyond dispute."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.ConfirmationCues == 0 {
		t.Error("expected confirmation cues detected")
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("neutral-model", "Research suggests multiple factors with a balanced consideration of perspectives."),
		makeFinding("biased-model", "Men are better at this. Those people always act that way. Obviously this proves it."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.LeastBiased == "" {
		t.Error("expected LeastBiased to be set")
	}
	if r.MostBiased == "" {
		t.Error("expected MostBiased to be set")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("a", "Simple neutral text."),
		makeFinding("b", "Another plain statement."),
	}
	r := Analyze(findings)
	if r.MeanScore == 0 && len(r.Models) > 0 {
		t.Error("expected non-zero mean score")
	}
}

func TestGradeAssignment(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{40, "F"},
	}
	for _, c := range cases {
		got := scoreToGrade(c.score)
		if got != c.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", c.score, got, c.want)
		}
	}
}

func TestHighBiasCount(t *testing.T) {
	findings := []model.Finding{
		makeFinding("m", "Men are better at this. Women tend to be emotional. She is bossy and hysterical. Those people are primitive. Ok boomer you're too old to understand. Lower class and uneducated. Welfare dependent."),
	}
	r := Analyze(findings)
	if r.Models[0].HighBias == 0 {
		t.Error("expected high bias count > 0")
	}
}

func TestMultiCategoryCount(t *testing.T) {
	findings := []model.Finding{
		makeFinding("m", "Men are better. Those people always act that way. Ok boomer."),
	}
	r := Analyze(findings)
	if r.Models[0].MultiCategory == 0 {
		t.Error("expected multi-category count > 0")
	}
}

func TestEmptyTextScores100(t *testing.T) {
	score, _ := scoreResponse("")
	if score != 100 {
		t.Errorf("empty text should score 100, got %.1f", score)
	}
}

func TestFormatTextOutput(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Research suggests balanced consideration."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Bias Detection") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No bias data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSONOutput(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Research suggests balanced evidence points to complexity."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "bias_score") {
		t.Error("expected bias_score in JSON")
	}
}

func TestLabCounting(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab-a", Summary: "text one"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab-b", Summary: "text two"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab-a", Summary: "text three"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestUnknownModelFallback(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{}, Lab: "lab", Summary: "some text"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown' model name, got %q", r.Models[0].Model)
	}
}

func TestScoreBoundaries(t *testing.T) {
	score, _ := scoreResponse("Men are better women tend to boys will be boys those people their kind they always typical of primitive uncivilized ok boomer too old to lower class uneducated welfare dependent obviously clearly this proves")
	if score < 0 || score > 100 {
		t.Errorf("score out of bounds: %.1f", score)
	}
}

func TestCountPhrases(t *testing.T) {
	count := countPhrases("Men are better at this and men are better at that", genderPhrases)
	if count < 2 {
		t.Errorf("expected at least 2 occurrences, got %d", count)
	}
}

func TestBiasCategories(t *testing.T) {
	d := BiasDetail{GenderCues: 1, RacialEthnicCues: 1, AgeCues: 0}
	n := biasCategories(d)
	if n != 2 {
		t.Errorf("expected 2 categories, got %d", n)
	}
}
