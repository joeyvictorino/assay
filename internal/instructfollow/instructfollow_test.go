package instructfollow

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, summary string) model.Finding {
	return model.Finding{
		ID:      "test-1",
		Summary: summary,
		Model:   model.ModelRef{Model: modelName},
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}
}

func TestSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "Response correctly follows all constraints"),
		mkFinding("gpt-4", "Output format matches specification"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "Response is complete"),
		mkFinding("claude", "Response ignored key instruction"),
		mkFinding("claude", "Partially fulfilled the request"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" {
		t.Error("expected non-empty best model")
	}
	if r.WorstModel == "" {
		t.Error("expected non-empty worst model")
	}
}

func TestIgnoredInstruction(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "Model ignored the constraint about length"),
	}
	r := Analyze(findings)
	ca := r.Models[0].Dimensions[0]
	if ca.Name != "constraint_adherence" {
		t.Errorf("expected constraint_adherence dimension, got %s", ca.Name)
	}
	if ca.Score >= 80 {
		t.Errorf("expected reduced score for ignored instruction, got %.1f", ca.Score)
	}
}

func TestContradiction(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "Response contradicts the given instruction"),
	}
	r := Analyze(findings)
	ca := r.Models[0].Dimensions[0]
	if ca.Score >= 50 {
		t.Errorf("expected low score for contradiction, got %.1f", ca.Score)
	}
}

func TestRefusal(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "Model refused to complete the task"),
	}
	r := Analyze(findings)
	tc := r.Models[0].Dimensions[2]
	if tc.Name != "task_completion" {
		t.Errorf("expected task_completion dimension, got %s", tc.Name)
	}
	if tc.Score >= 50 {
		t.Errorf("expected low score for refusal, got %.1f", tc.Score)
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models for nil findings, got %d", len(r.Models))
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "Good response"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if text == "" {
		t.Error("expected non-empty text output")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "Good response"),
	}
	r := Analyze(findings)
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

func TestGrading(t *testing.T) {
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
		if g := gradeFromScore(tt.score); g != tt.grade {
			t.Errorf("score %.0f: expected grade %s, got %s", tt.score, tt.grade, g)
		}
	}
}
