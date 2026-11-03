package objectivity

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, lab, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     lab,
		Summary: summary,
	}
}

func TestAnalyzeTextObjective(t *testing.T) {
	text := "According to the specification, the function returns 200 for valid input. Based on benchmarks, the endpoint produces 1500 requests per second. The documentation states the API accepts JSON."
	score, detail := AnalyzeText(text)
	if score <= 50 {
		t.Errorf("objective text should score above 50, got %.1f", score)
	}
	if detail.NeutralCount == 0 {
		t.Error("expected neutral phrases")
	}
	if detail.FactualCount == 0 {
		t.Error("expected factual phrases")
	}
}

func TestAnalyzeTextBiased(t *testing.T) {
	text := "Obviously this is terrible. I personally hate this approach. Everyone knows you should always use the other library. It's amazing how awful this is. I strongly believe this is the worst ever."
	score, detail := AnalyzeText(text)
	if score >= 50 {
		t.Errorf("biased text should score below 50, got %.1f", score)
	}
	if detail.BiasCount == 0 {
		t.Error("expected bias phrases")
	}
	if detail.OpinionCount == 0 {
		t.Error("expected opinion phrases")
	}
	if detail.EmotionalCount == 0 {
		t.Error("expected emotional phrases")
	}
}

func TestAnalyzeTextEmpty(t *testing.T) {
	score, detail := AnalyzeText("")
	if score != 50.0 {
		t.Errorf("empty text should score 50, got %.1f", score)
	}
	if detail.WordCount != 0 {
		t.Errorf("empty text should have 0 words, got %d", detail.WordCount)
	}
}

func TestNeutralDetection(t *testing.T) {
	text := "According to the docs, based on measurements, data shows improvement. The specification defines the protocol. One approach is to use caching."
	count := countPhrases(text, neutralPhrases)
	if count < 4 {
		t.Errorf("expected at least 4 neutral phrases, got %d", count)
	}
}

func TestBiasDetection(t *testing.T) {
	text := "Obviously the best approach is always to rewrite. Everyone knows this. You should always prefer Go. Never use Python. You must never ignore this."
	count := countPhrases(text, biasPhrases)
	if count < 4 {
		t.Errorf("expected at least 4 bias phrases, got %d", count)
	}
}

func TestOpinionDetection(t *testing.T) {
	text := "I personally prefer this. In my opinion it's better. I feel that we should change. I hate the current design. To be honest, I love the new API."
	count := countPhrases(text, opinionPhrases)
	if count < 4 {
		t.Errorf("expected at least 4 opinion phrases, got %d", count)
	}
}

func TestEmotionalDetection(t *testing.T) {
	text := "This is amazing and fantastic. The old code is terrible and horrible. It's incredibly awful. Totally broken and the worst ever."
	count := countPhrases(text, emotionalPhrases)
	if count < 5 {
		t.Errorf("expected at least 5 emotional phrases, got %d", count)
	}
}

func TestQualifiedDetection(t *testing.T) {
	text := "In this context, depending on the load. For this use case, given that latency matters. Under these conditions, when performance matters."
	count := countPhrases(text, qualifiedPhrases)
	if count < 4 {
		t.Errorf("expected at least 4 qualified phrases, got %d", count)
	}
}

func TestFactualDetection(t *testing.T) {
	text := "The function returns a list. The method takes two arguments. The API accepts JSON and produces XML. The endpoint requires authentication."
	count := countPhrases(text, factualPhrases)
	if count < 5 {
		t.Errorf("expected at least 5 factual phrases, got %d", count)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {40, "F"},
	}
	for _, tt := range tests {
		if got := scoreToGrade(tt.score); got != tt.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestScoreClamping(t *testing.T) {
	biased := "obviously terrible awful horrible disgusting " +
		"i hate i personally i love amazing fantastic incredible " +
		"best ever worst ever totally broken insanely ridiculously"
	score, _ := AnalyzeText(biased)
	if score < 0 || score > 100 {
		t.Errorf("score should be clamped to [0,100], got %.1f", score)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "According to the specification, the function returns 200. Based on data, the method takes 3 parameters."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("expected best model gpt-4, got %s", r.BestModel)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "According to the documentation, the function returns JSON with 5 fields. Based on benchmarks, it produces 1000 results per second."),
		mkFinding("claude", "lab1", "I personally hate this terrible approach. Obviously everyone knows it's the worst ever. I strongly believe we should always prefer something else."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ with different quality texts")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("", "lab1", "The function returns a list of 7 items."),
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(r.Models))
	}
}

func TestLabCounting(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "The function returns 10 items."),
		mkFinding("gpt-4", "lab2", "According to the spec, 20 records."),
		mkFinding("gpt-4", "lab1", "The method takes 5 args."),
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
	if r.Models[0].SampleCount != 3 {
		t.Errorf("expected 3 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "According to the spec, the function returns 3 values."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Objectivity Analysis") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No objectivity data") {
		t.Error("missing empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "The API returns JSON data."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "gpt-4") {
		t.Error("expected model in JSON output")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", "According to the docs, the function returns 10 items."),
		mkFinding("m2", "lab1", "I personally hate this terrible code."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestObjectivityRatio(t *testing.T) {
	_, detail := AnalyzeText("According to the data, the function returns results. I personally hate this terrible approach.")
	if detail.ObjectivityRatio <= 0 || detail.ObjectivityRatio >= 1 {
		t.Errorf("objectivity ratio should be between 0 and 1 exclusive, got %.2f", detail.ObjectivityRatio)
	}
}
