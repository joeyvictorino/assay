package persuasiveness

import (
	"math"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestCountPhrases(t *testing.T) {
	tests := []struct {
		text    string
		phrases []string
		want    int
	}{
		{"because this therefore works", argumentPhrases, 2},
		{"nothing here", argumentPhrases, 0},
		{"According to the data, based on analysis", evidencePhrases, 2},
		{"first second third", logicPhrases, 3},
		{"consider the impact of imagine if", rhetoricalPhrases, 2},
		{"although some argue and critics might say", counterPhrases, 2},
		{"i think i guess maybe", weakPhrases, 2},
		{"sort of kind of arguably", hedgingPhrases, 3},
	}
	for _, tt := range tests {
		got := countPhrases(tt.text, tt.phrases)
		if got != tt.want {
			t.Errorf("countPhrases(%q) = %d, want %d", tt.text[:20], got, tt.want)
		}
	}
}

func TestCountPhrasesCase(t *testing.T) {
	got := countPhrases("BECAUSE Therefore CONSEQUENTLY", argumentPhrases)
	if got != 3 {
		t.Errorf("case insensitive count = %d, want 3", got)
	}
}

func TestScoreResponseBaseline(t *testing.T) {
	score, detail := scoreResponse("hello world", "")
	if score < 30 || score > 60 {
		t.Errorf("baseline score = %.1f, want 30-60", score)
	}
	if detail.PersuasiveCount != 0 {
		t.Errorf("persuasive count = %d, want 0", detail.PersuasiveCount)
	}
}

func TestScoreResponsePersuasive(t *testing.T) {
	text := "Because the evidence shows, therefore consequently as a result. " +
		"According to research, based on data, demonstrated by experiments. " +
		"First, furthermore, moreover, in conclusion. " +
		"Consider the impact, significantly, fundamentally. " +
		"Although some argue, critics might say, the counterargument."
	score, detail := scoreResponse(text, "")
	if score < 60 {
		t.Errorf("persuasive score = %.1f, want >= 60", score)
	}
	if detail.PersuasiveCount < 10 {
		t.Errorf("persuasive count = %d, want >= 10", detail.PersuasiveCount)
	}
}

func TestScoreResponseWeak(t *testing.T) {
	text := "I think maybe i guess i believe not sure but i could be wrong. " +
		"Sort of kind of more or less to some extent arguably seemingly."
	_, detail := scoreResponse(text, "")
	if detail.WeakCount < 4 {
		t.Errorf("weak count = %d, want >= 4", detail.WeakCount)
	}
	if detail.HedgingCount < 4 {
		t.Errorf("hedging count = %d, want >= 4", detail.HedgingCount)
	}
}

func TestScoreResponseClamped(t *testing.T) {
	score, _ := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Errorf("score = %.1f, want 0-100", score)
	}
}

func TestScoreResponseRounding(t *testing.T) {
	score, detail := scoreResponse("because therefore", "")
	remainder := math.Mod(score*10, 1.0)
	if remainder > 0.01 {
		t.Errorf("score %.1f not rounded to 1 decimal", score)
	}
	r2 := math.Mod(detail.ArgumentStrength*100, 1.0)
	if r2 > 0.01 {
		t.Errorf("argument strength %.2f not rounded to 2 decimals", detail.ArgumentStrength)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{95, "A"}, {90, "A"},
		{85, "B"}, {80, "B"},
		{75, "C"}, {70, "C"},
		{65, "D"}, {60, "D"},
		{55, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%.0f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestEvidenceTextMarkers(t *testing.T) {
	f := model.Finding{
		Summary: "fallback",
		Evidence: []model.Evidence{
			{Marker: "alpha"},
			{Marker: "beta"},
		},
	}
	got := evidenceText(f)
	if got != "alpha beta" {
		t.Errorf("evidenceText = %q, want %q", got, "alpha beta")
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "my summary"}
	got := evidenceText(f)
	if got != "my summary" {
		t.Errorf("evidenceText = %q, want %q", got, "my summary")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("empty analyze returned %d models", len(r.Models))
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "gpt-4"}, Summary: "Because therefore the evidence shows", Lab: "lab1"},
		{Model: model.ModelRef{Model: "gpt-4"}, Summary: "Furthermore moreover in conclusion", Lab: "lab2"},
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("got %d models, want 1", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("model = %q, want gpt-4", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("sample count = %d, want 2", r.Models[0].SampleCount)
	}
	if r.Models[0].LabCount != 2 {
		t.Errorf("lab count = %d, want 2", r.Models[0].LabCount)
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("best model = %q, want gpt-4", r.BestModel)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "strong"}, Summary: "Because therefore consequently as a result the evidence shows research indicates studies show", Lab: "a"},
		{Model: model.ModelRef{Model: "weak"}, Summary: "I think i guess not sure but i could be wrong sort of kind of", Lab: "a"},
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("got %d models, want 2", len(r.Models))
	}
	if r.BestModel != "strong" {
		t.Errorf("best = %q, want strong", r.BestModel)
	}
	if r.WorstModel != "weak" {
		t.Errorf("worst = %q, want weak", r.WorstModel)
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "because therefore", Lab: "x"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("model = %q, want unknown", r.Models[0].Model)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m"}, Summary: "I think i guess i believe not sure but i could be wrong this is just my opinion take this with a grain", Lab: "a"},
	}
	r := Analyze(findings)
	if r.Models[0].Concerns != 1 {
		t.Errorf("concerns = %d, want 1", r.Models[0].Concerns)
	}
}

func TestAnalyzeSorted(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "low"}, Summary: "i think i guess sort of kind of", Lab: "a"},
		{Model: model.ModelRef{Model: "high"}, Summary: "because therefore consequently as a result the evidence shows", Lab: "a"},
	}
	r := Analyze(findings)
	if r.Models[0].PersuasivenessScore < r.Models[1].PersuasivenessScore {
		t.Error("models not sorted descending by score")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Report{}
	out := FormatText(r)
	if !strings.Contains(out, "No persuasiveness data") {
		t.Error("empty report missing expected message")
	}
}

func TestFormatTextContent(t *testing.T) {
	r := Report{
		Models: []ModelPersuasiveness{{
			Model: "test", Grade: "B", PersuasivenessScore: 82.5,
			SampleCount: 3, LabCount: 2, Concerns: 1,
			Detail: PersuasivenessDetail{
				ArgumentStrength: 0.6, EvidenceUse: 0.5,
				LogicalFlow: 0.7, RhetoricalSkill: 0.4, CounterHandling: 0.3,
				PersuasiveCount: 8, WeakCount: 2, HedgingCount: 1,
			},
		}},
		BestModel: "test", WorstModel: "test", MeanScore: 82.5,
	}
	out := FormatText(r)
	for _, want := range []string{"Persuasiveness", "test", "[B]", "82.5", "argument_strength", "evidence_use", "Concerns: 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatText missing %q", want)
		}
	}
}

func TestFormatJSON(t *testing.T) {
	r := Report{
		Models:    []ModelPersuasiveness{{Model: "m1", PersuasivenessScore: 70}},
		BestModel: "m1", WorstModel: "m1", MeanScore: 70,
	}
	out := FormatJSON(r)
	if !strings.Contains(out, `"persuasiveness_score"`) {
		t.Error("JSON missing persuasiveness_score key")
	}
	if !strings.Contains(out, `"m1"`) {
		t.Error("JSON missing model name")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "a"}, Summary: "because therefore", Lab: "x"},
		{Model: model.ModelRef{Model: "b"}, Summary: "i think i guess", Lab: "x"},
	}
	r := Analyze(findings)
	expected := math.Round((r.Models[0].PersuasivenessScore+r.Models[1].PersuasivenessScore)/2*10) / 10
	if r.MeanScore != expected {
		t.Errorf("mean = %.1f, want %.1f", r.MeanScore, expected)
	}
}
