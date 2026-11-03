package concreteness

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Summary: summary,
		Lab:     "test-lab",
	}
}

func makeFindingWithEvidence(modelName, summary string, markers []string) model.Finding {
	f := makeFinding(modelName, summary)
	for _, m := range markers {
		f.Evidence = append(f.Evidence, model.Evidence{Marker: m})
	}
	return f
}

func makeFindingLab(modelName, summary, lab string) model.Finding {
	f := makeFinding(modelName, summary)
	f.Lab = lab
	return f
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "For example, specifically the output shows correct results."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.BestModel)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("model-a", "Specifically, for example, the test result shows 95 percent accuracy."),
		makeFinding("model-b", "Generally it usually works with various things and stuff."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].ConcreteScore < r.Models[1].ConcreteScore {
		t.Error("models should be sorted descending by score")
	}
}

func TestSpecificityPhrases(t *testing.T) {
	specific := "Specifically, for example, such as the exact result, namely the correct one."
	neutral := "The result is correct."
	sS, dS := scoreResponse(specific, "")
	sN, _ := scoreResponse(neutral, "")
	if sS <= sN {
		t.Errorf("specific text (%.1f) should score higher than neutral (%.1f)", sS, sN)
	}
	if dS.SpecificCount == 0 {
		t.Error("expected specific count > 0")
	}
}

func TestVaguePhrasesReduce(t *testing.T) {
	vague := "Generally this usually works. Typically it is broadly, roughly, approximately correct. More or less some kind of a type of various stuff."
	direct := "The function returns 42."
	sV, dV := scoreResponse(vague, "")
	sD, _ := scoreResponse(direct, "")
	if sV >= sD {
		t.Errorf("vague text (%.1f) should score lower than direct (%.1f)", sV, sD)
	}
	if dV.VagueCount == 0 {
		t.Error("expected vague count > 0")
	}
}

func TestExamplePhrases(t *testing.T) {
	text := "For instance, consider this: for example, to illustrate the point."
	_, d := scoreResponse(text, "")
	if d.ExampleScore <= 0.2 {
		t.Errorf("example-rich text should have example score > 0.2, got %.2f", d.ExampleScore)
	}
	if d.ExampleCount == 0 {
		t.Error("expected example count > 0")
	}
}

func TestQuantitativePhrases(t *testing.T) {
	text := "The rate of failure is 5 percent, measured at a total of 100 runs."
	_, d := scoreResponse(text, "")
	if d.QuantitativeScore <= 0.2 {
		t.Errorf("quantitative text should have score > 0.2, got %.2f", d.QuantitativeScore)
	}
}

func TestActionablePhrases(t *testing.T) {
	text := "First, run the command to install the package. Then, configure the settings."
	_, d := scoreResponse(text, "")
	if d.ActionableScore <= 0.2 {
		t.Errorf("actionable text should have score > 0.2, got %.2f", d.ActionableScore)
	}
}

func TestEvidencePhrases(t *testing.T) {
	text := "According to the documentation shows the output shows correct results."
	_, d := scoreResponse(text, "")
	if d.EvidenceScore <= 0.2 {
		t.Errorf("evidence text should have score > 0.2, got %.2f", d.EvidenceScore)
	}
}

func TestScoreBounds(t *testing.T) {
	texts := []string{
		"",
		"Specifically for example such as namely including e.g. precisely the exact.",
		"Generally usually often sometimes typically broadly roughly approximately stuff.",
	}
	for _, text := range texts {
		score, _ := scoreResponse(text, "")
		if score < 0 || score > 100 {
			t.Errorf("score %.1f out of bounds for: %q", score, text[:min(50, len(text))])
		}
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
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := makeFinding("m", "summary text")
	got := evidenceText(f)
	if got != "summary text" {
		t.Errorf("expected summary fallback, got %q", got)
	}
}

func TestEvidenceTextMarkers(t *testing.T) {
	f := makeFindingWithEvidence("m", "summary", []string{"marker1", "marker2"})
	got := evidenceText(f)
	if got != "marker1 marker2" {
		t.Errorf("expected joined markers, got %q", got)
	}
}

func TestConcernCountThreshold(t *testing.T) {
	vague := "Generally usually often sometimes typically broadly roughly approximately more or less stuff."
	findings := []model.Finding{makeFinding("m", vague)}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns for heavily vague response")
	}
}

func TestNoConcernForLowVagueness(t *testing.T) {
	text := "The answer is specifically 42."
	findings := []model.Finding{makeFinding("m", text)}
	r := Analyze(findings)
	if r.Models[0].Concerns != 0 {
		t.Errorf("expected 0 concerns, got %d", r.Models[0].Concerns)
	}
}

func TestLabCounting(t *testing.T) {
	findings := []model.Finding{
		makeFindingLab("m", "text one", "lab-a"),
		makeFindingLab("m", "text two", "lab-b"),
		makeFindingLab("m", "text three", "lab-a"),
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestUnknownModelName(t *testing.T) {
	f := model.Finding{Summary: "test", Lab: "l"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %q", r.Models[0].Model)
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("a", "Specifically the exact result for example."),
		makeFinding("b", "Generally stuff usually works."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("mean score should be > 0")
	}
}

func TestFormatTextHeader(t *testing.T) {
	findings := []model.Finding{makeFinding("m", "Specifically correct.")}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Concreteness") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "specificity_score") {
		t.Error("expected specificity_score in text output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No concreteness data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{makeFinding("m", "Test.")}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "concrete_score") {
		t.Error("expected concrete_score in JSON")
	}
}

func TestCountPhrases(t *testing.T) {
	text := "specifically for example specifically"
	count := countPhrases(text, specificityPhrases)
	if count < 2 {
		t.Errorf("expected at least 2, got %d", count)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
