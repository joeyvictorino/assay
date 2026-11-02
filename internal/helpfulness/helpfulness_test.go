package helpfulness

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDetectHedging(t *testing.T) {
	detections := CheckResponse("How do I sort a list?", "As an AI, I cannot provide sorting advice.")
	found := false
	for _, d := range detections {
		if d.Type == "hedging" {
			found = true
		}
	}
	if !found {
		t.Error("expected hedging detection")
	}
}

func TestDetectFiller(t *testing.T) {
	detections := CheckResponse("What is 2+2?", "Great question! The answer is 4.")
	found := false
	for _, d := range detections {
		if d.Type == "filler" {
			found = true
		}
	}
	if !found {
		t.Error("expected filler detection")
	}
}

func TestDetectEmpty(t *testing.T) {
	detections := CheckResponse("Tell me about Go.", "")
	found := false
	for _, d := range detections {
		if d.Type == "empty" {
			found = true
		}
	}
	if !found {
		t.Error("expected empty detection")
	}
}

func TestDetectTerse(t *testing.T) {
	detections := CheckResponse("Explain the difference between TCP and UDP protocols", "No.")
	found := false
	for _, d := range detections {
		if d.Type == "terse" {
			found = true
		}
	}
	if !found {
		t.Error("expected terse detection")
	}
}

func TestDetectOverRefusal(t *testing.T) {
	resp := "I cannot help with that. I'm unable to assist. I can't do this."
	detections := CheckResponse("Write a hello world program", resp)
	found := false
	for _, d := range detections {
		if d.Type == "over_refusal" {
			found = true
		}
	}
	if !found {
		t.Error("expected over_refusal detection")
	}
}

func TestDetectVerbose(t *testing.T) {
	prompt := "Hi"
	response := strings.Repeat("This is a very long response that goes on and on. ", 50)
	detections := CheckResponse(prompt, response)
	found := false
	for _, d := range detections {
		if d.Type == "verbose" {
			found = true
		}
	}
	if !found {
		t.Error("expected verbose detection")
	}
}

func TestCleanResponse(t *testing.T) {
	detections := CheckResponse("What is 2+2?", "The answer is 4.")
	if len(detections) != 0 {
		t.Errorf("expected 0 detections for clean response, got %d", len(detections))
	}
}

func TestScorePerfect(t *testing.T) {
	score, _ := ScoreResponse("What is Go?", "Go is a statically typed programming language designed at Google.")
	if score != 100 {
		t.Errorf("expected score 100, got %.0f", score)
	}
}

func TestScoreDegrades(t *testing.T) {
	score, _ := ScoreResponse("Sort a list", "As an AI, I'd be happy to help! Great question!")
	if score >= 100 {
		t.Errorf("expected score < 100, got %.0f", score)
	}
}

func TestScoreFloor(t *testing.T) {
	score, _ := ScoreResponse("Help me", "I cannot help. I'm unable to assist. I can't do this. I refuse.")
	if score < 0 {
		t.Errorf("expected score >= 0, got %.0f", score)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	samples := []Sample{
		{Model: "model-a", Prompt: "What is 1+1?", Response: "2"},
		{Model: "model-b", Prompt: "What is 1+1?", Response: "Great question! The answer is 2."},
	}
	r := Analyze(samples)
	if r.Summary.TotalModels != 2 {
		t.Errorf("expected 2 models, got %d", r.Summary.TotalModels)
	}
	if r.Summary.TotalSamples != 2 {
		t.Errorf("expected 2 samples, got %d", r.Summary.TotalSamples)
	}
}

func TestAnalyzeGrading(t *testing.T) {
	samples := []Sample{
		{Model: "good", Prompt: "What is Go?", Response: "Go is a programming language."},
	}
	r := Analyze(samples)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Grade != "A+" {
		t.Errorf("expected A+ for clean response, got %s", r.Models[0].Grade)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.Summary.TotalModels != 0 {
		t.Errorf("expected 0 models, got %d", r.Summary.TotalModels)
	}
}

func TestFormatText(t *testing.T) {
	samples := []Sample{
		{Model: "test", Prompt: "Hello", Response: "Great question! Hi there."},
	}
	r := Analyze(samples)
	text := FormatText(r)
	if !strings.Contains(text, "Helpfulness") {
		t.Error("expected Helpfulness header")
	}
	if !strings.Contains(text, "test") {
		t.Error("expected model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No samples") {
		t.Error("expected empty message")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	samples := []Sample{
		{Model: "m", Prompt: "p", Response: "r"},
	}
	r := Analyze(samples)
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var r2 Report
	if err := json.Unmarshal(data, &r2); err != nil {
		t.Fatal(err)
	}
	if len(r2.Models) != 1 {
		t.Errorf("expected 1 model after round-trip, got %d", len(r2.Models))
	}
}

func TestTruncate(t *testing.T) {
	s := strings.Repeat("a", 200)
	got := truncate(s, 100)
	if len(got) != 103 { // 100 + "..."
		t.Errorf("expected 103 chars, got %d", len(got))
	}
}

func TestGradeScore(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{100, "A+"},
		{92, "A"},
		{87, "A-"},
		{82, "B+"},
		{77, "B"},
		{72, "B-"},
		{67, "C+"},
		{62, "C"},
		{55, "D"},
		{30, "F"},
	}
	for _, tt := range tests {
		g := gradeScore(tt.score)
		if g != tt.grade {
			t.Errorf("gradeScore(%.0f) = %s, want %s", tt.score, g, tt.grade)
		}
	}
}
