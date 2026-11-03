package jargon

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(mn, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mn},
		Summary: summary,
		Lab:     "lab1",
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected empty models, got %d", len(r.Models))
	}
}

func TestClearResponse(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "The dog ran across the park and played with the ball."),
	})
	if r.Models[0].JargonScore < 70 {
		t.Errorf("clear response should score high, got %.1f", r.Models[0].JargonScore)
	}
	if r.Models[0].Detail.ReadabilityHint != "clear" {
		t.Errorf("expected 'clear' hint, got %s", r.Models[0].Detail.ReadabilityHint)
	}
}

func TestJargonHeavyResponse(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "The microservice uses kubernetes orchestration with containerization and API endpoint for asynchronous middleware deployment via webhook and graphql proxy with OAuth authentication."),
	})
	if r.Models[0].JargonScore >= 70 {
		t.Errorf("jargon-heavy response should score low, got %.1f", r.Models[0].JargonScore)
	}
	if r.Models[0].Detail.JargonWords == 0 {
		t.Error("expected jargon words to be detected")
	}
}

func TestAcronymDetection(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "Use the API with REST and SDK for DNS and TLS via HTTP over TCP."),
	})
	if r.Models[0].Detail.AcronymCount == 0 {
		t.Error("expected acronyms to be detected")
	}
}

func TestLongWordCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "The interoperability and containerization of this authentication requires deserialization."),
	})
	if r.Models[0].Detail.LongWordCount == 0 {
		t.Error("expected long words to be detected")
	}
}

func TestMultipleModels(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("clear-model", "The cat sat on the mat and looked out the window."),
		finding("jargon-model", "Deploy the microservice cluster with kubernetes orchestration and containerization for API scalability."),
	})
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ")
	}
}

func TestGradeMapping(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{45, "F"},
	}
	for _, tt := range tests {
		if got := scoreToGrade(tt.score); got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Analyze(nil))
	if !strings.Contains(text, "No jargon data") {
		t.Error("empty report should say no data")
	}
}

func TestFormatTextHeader(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "test")})
	text := FormatText(r)
	if !strings.Contains(text, "Response Jargon Analysis") {
		t.Error("missing report header")
	}
}

func TestUnknownModelName(t *testing.T) {
	r := Analyze([]model.Finding{
		{Summary: "test response", Lab: "lab1"},
	})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestLabCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab2"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
	})
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestSampleCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "a"),
		finding("m1", "b"),
		finding("m1", "c"),
	})
	if r.Models[0].SampleCount != 3 {
		t.Errorf("expected 3 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "The cat sat on the mat."),
		finding("m2", "Deploy kubernetes microservice API."),
	})
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestJargonDensityCalculation(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "The API uses OAuth authentication for endpoint authorization with middleware proxy."),
	})
	if r.Models[0].Detail.JargonDensity <= 0 {
		t.Errorf("expected positive jargon density, got %.3f", r.Models[0].Detail.JargonDensity)
	}
}

func TestScoreClamping(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "The cat is a nice cat."),
	})
	if r.Models[0].JargonScore > 100 {
		t.Errorf("score should be capped at 100, got %.1f", r.Models[0].JargonScore)
	}
	if r.Models[0].JargonScore < 0 {
		t.Errorf("score should not go below 0, got %.1f", r.Models[0].JargonScore)
	}
}

func TestIsAcronym(t *testing.T) {
	tests := []struct {
		word string
		want bool
	}{
		{"API", true},
		{"HTTP", true},
		{"a", false},
		{"Hello", false},
		{"TOOLONG!", false},
		{"AB", true},
	}
	for _, tt := range tests {
		if got := isAcronym(tt.word); got != tt.want {
			t.Errorf("isAcronym(%q) = %v, want %v", tt.word, got, tt.want)
		}
	}
}
