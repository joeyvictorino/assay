package contextadherence

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
	if r.MeanScore != 0 {
		t.Fatalf("expected 0 mean, got %.1f", r.MeanScore)
	}
}

func TestSingleFinding(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary:  "Kubernetes provides container orchestration with pods and services.",
			Model:    model.ModelRef{Model: "gpt-4"},
			Location: model.Location{PathTemplate: "Explain Kubernetes container orchestration pods services deployments."},
		},
	}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Fatalf("expected gpt-4, got %s", r.Models[0].Model)
	}
}

func TestHighAdherence(t *testing.T) {
	context := "Python Django framework provides web development with models views templates URL routing and middleware support for building scalable applications"
	response := "Python Django framework provides web development capabilities through models, views, and templates. URL routing connects requests to views, and middleware support enables scalable applications."
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1", Summary: response,
			Model:    model.ModelRef{Model: "claude"},
			Location: model.Location{PathTemplate: context},
		},
	}
	r := Analyze(fs)
	if r.Models[0].AdherenceScore < 60 {
		t.Fatalf("expected high adherence, got %.1f", r.Models[0].AdherenceScore)
	}
}

func TestLowAdherence(t *testing.T) {
	context := "Python Django framework web development"
	response := "Kubernetes provides container orchestration across distributed cloud infrastructure with automatic scaling horizontal pod autoscaler load balancing ingress controllers service mesh integration monitoring observability"
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1", Summary: response,
			Model:    model.ModelRef{Model: "gpt-3"},
			Location: model.Location{PathTemplate: context},
		},
	}
	r := Analyze(fs)
	if r.Models[0].AdherenceScore >= 70 {
		t.Fatalf("expected low adherence for off-topic response, got %.1f", r.Models[0].AdherenceScore)
	}
}

func TestKeywordCoverage(t *testing.T) {
	total, used, cov := keywordCoverage(
		"machine learning neural networks deep learning training inference",
		"machine learning uses neural networks for deep learning training",
	)
	if total < 4 {
		t.Fatalf("expected at least 4 context keywords, got %d", total)
	}
	if used < 3 {
		t.Fatalf("expected at least 3 used keywords, got %d", used)
	}
	if cov < 0.5 {
		t.Fatalf("expected coverage > 0.5, got %.3f", cov)
	}
}

func TestEmptyContextCoverage(t *testing.T) {
	total, _, cov := keywordCoverage("", "some response text")
	if total != 0 {
		t.Fatalf("expected 0 total for empty context, got %d", total)
	}
	if cov != 1.0 {
		t.Fatalf("expected 1.0 coverage for empty context, got %.3f", cov)
	}
}

func TestHallucinationDetection(t *testing.T) {
	context := "Python is a programming language."
	response := "As everyone knows, Python is great. Studies show it is popular. Research has proven its effectiveness. According to experts, it leads the field."
	count := detectHallucinations(context, response)
	if count < 3 {
		t.Fatalf("expected at least 3 hallucination indicators, got %d", count)
	}
}

func TestNoHallucinations(t *testing.T) {
	context := "Studies show Python is popular. Research has proven it is effective."
	response := "Studies show Python is popular. Research has proven it is effective."
	count := detectHallucinations(context, response)
	if count > 0 {
		t.Fatalf("expected 0 hallucinations when response matches context, got %d", count)
	}
}

func TestContextDrift(t *testing.T) {
	context := "database indexing query optimization performance tuning"
	response := "weather forecasting climate patterns precipitation humidity atmospheric pressure temperature gradients wind speed measurements"
	drift := contextDrift(context, response)
	if drift < 0.5 {
		t.Fatalf("expected high drift for unrelated response, got %.3f", drift)
	}
}

func TestLowContextDrift(t *testing.T) {
	context := "database indexing query optimization performance tuning"
	response := "database indexing improves query optimization for better performance tuning"
	drift := contextDrift(context, response)
	if drift > 0.5 {
		t.Fatalf("expected low drift for on-topic response, got %.3f", drift)
	}
}

func TestSelectiveUsage(t *testing.T) {
	context := "important important important important rare unique special"
	response := "important important important important"
	bias := selectiveUsage(context, response)
	if bias < 0 {
		t.Fatal("selectiveUsage should be non-negative")
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary:  "Python provides web development with Django framework.",
			Model:    model.ModelRef{Model: "claude"},
			Location: model.Location{PathTemplate: "Python Django web development framework"},
		},
		{
			ID: "f2", Lab: "lab-1",
			Summary:  "Kubernetes container orchestration cloud infrastructure.",
			Model:    model.ModelRef{Model: "gpt-4"},
			Location: model.Location{PathTemplate: "Python Django web development framework"},
		},
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestScoreBounds(t *testing.T) {
	pairs := []struct{ ctx, resp string }{
		{"a", "b"},
		{"machine learning deep neural networks training inference optimization", "machine learning uses deep neural networks for training and inference optimization"},
		{"x", "As everyone knows, studies show, research has proven, according to experts, data suggests, statistics indicate, it is well known that"},
	}
	for _, p := range pairs {
		score, _ := ScoreResponse(p.ctx, p.resp)
		if score < 0 || score > 100 {
			t.Fatalf("score %.1f out of bounds", score)
		}
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {45, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestFormatText(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary:  "Python provides web development.",
			Model:    model.ModelRef{Model: "test-model"},
			Location: model.Location{PathTemplate: "Python web development"},
		},
	}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Context Adherence") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "test-model") {
		t.Fatal("expected model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No adherence data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary:  "test response",
			Model:    model.ModelRef{Model: "m1"},
			Location: model.Location{PathTemplate: "test context"},
		},
	}
	r := Analyze(fs)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Fatal("expected models key")
	}
}

func TestUnknownModel(t *testing.T) {
	fs := []model.Finding{
		{ID: "f1", Lab: "lab-1", Summary: "test", Model: model.ModelRef{}},
	}
	r := Analyze(fs)
	if r.Models[0].Model != "unknown" {
		t.Fatalf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestLabCounting(t *testing.T) {
	fs := []model.Finding{
		{ID: "f1", Lab: "lab-1", Summary: "a", Model: model.ModelRef{Model: "m1"}, Location: model.Location{PathTemplate: "ctx"}},
		{ID: "f2", Lab: "lab-2", Summary: "b", Model: model.ModelRef{Model: "m1"}, Location: model.Location{PathTemplate: "ctx"}},
		{ID: "f3", Lab: "lab-1", Summary: "c", Model: model.ModelRef{Model: "m1"}, Location: model.Location{PathTemplate: "ctx"}},
	}
	r := Analyze(fs)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestEmptyResponseScore(t *testing.T) {
	score, _ := ScoreResponse("some context words here", "")
	if score > 60 {
		t.Fatalf("expected low score for empty response, got %.1f", score)
	}
}
