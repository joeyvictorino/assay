package groundedness

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(modelName, summary string, evidence ...string) model.Finding {
	var evs []model.Evidence
	for _, e := range evidence {
		evs = append(evs, model.Evidence{Marker: e})
	}
	return model.Finding{
		Model:    model.ModelRef{Model: modelName},
		Summary:  summary,
		Evidence: evs,
		Lab:      "test-lab",
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected empty report")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("gpt-4", "The server uses port 443 for HTTPS.", "port 443 HTTPS server configuration"),
	})
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].GroundednessScore <= 0 {
		t.Error("expected positive score")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("gpt-4", "Exact match text.", "Exact match text."),
		finding("claude", "Different unrelated content.", "source evidence here"),
	})
	if len(r.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best/worst model names")
	}
}

func TestGroundedResponseHighScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("model-a", "The database runs on port 5432 with TLS enabled.",
			"database port 5432 TLS enabled configuration PostgreSQL"),
	})
	if r.Models[0].GroundednessScore < 50 {
		t.Errorf("well-grounded response should score above 50, got %.1f", r.Models[0].GroundednessScore)
	}
}

func TestHallucinatedResponseLowScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("model-b",
			"According to studies, research shows that experts say this is well known. Statistics show surveys indicate data suggests it is possible that.",
			"simple fact"),
	})
	if r.Models[0].GroundednessScore > 60 {
		t.Errorf("hallucinated response should score below 60, got %.1f", r.Models[0].GroundednessScore)
	}
}

func TestGradeMapping(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {45, "F"},
	}
	for _, tt := range tests {
		if got := scoreToGrade(tt.score); got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No groundedness data available") {
		t.Error("expected empty message")
	}
}

func TestFormatTextHeader(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "test summary", "test evidence"),
	})
	text := FormatText(r)
	if !strings.Contains(text, "Response Groundedness Analysis") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("missing model name")
	}
}

func TestUnknownModelName(t *testing.T) {
	r := Analyze([]model.Finding{
		{Summary: "test", Lab: "lab1"},
	})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestLabCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "a", Lab: "lab-1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "b", Lab: "lab-2"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "c", Lab: "lab-1"},
	})
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestSampleCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "a", "x"),
		finding("m1", "b", "y"),
	})
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "test one", "evidence one"),
		finding("m2", "test two", "evidence two"),
	})
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestWordOverlap(t *testing.T) {
	overlap := wordOverlap("the quick brown fox jumps", "the quick brown fox leaps")
	if overlap < 0.5 {
		t.Errorf("expected high overlap, got %.2f", overlap)
	}
}

func TestWordOverlapNoMatch(t *testing.T) {
	overlap := wordOverlap("alpha beta gamma", "delta epsilon zeta")
	if overlap > 0.1 {
		t.Errorf("expected low overlap, got %.2f", overlap)
	}
}
