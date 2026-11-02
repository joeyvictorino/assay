package boundarytest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, detail string) model.Finding {
	return model.Finding{
		Model:  model.ModelRef{Model: modelName},
		Summary: detail,
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestSingleModelSystemPromptRespected(t *testing.T) {
	fs := []model.Finding{
		mkFinding("gpt-4", "system prompt respected by model"),
		mkFinding("gpt-4", "system boundary maintained in response"),
	}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].OverallScore < 50 {
		t.Errorf("expected score >= 50, got %.1f", r.Models[0].OverallScore)
	}
}

func TestSystemPromptViolation(t *testing.T) {
	fs := []model.Finding{
		mkFinding("gpt-4", "system prompt violated by user manipulation"),
		mkFinding("gpt-4", "system prompt violated again"),
	}
	r := Analyze(fs)
	if r.Models[0].Dimensions[0].Score >= 50 {
		t.Errorf("expected system_prompt_adherence < 50 for violations, got %.1f", r.Models[0].Dimensions[0].Score)
	}
}

func TestRoleEscaped(t *testing.T) {
	fs := []model.Finding{
		mkFinding("claude", "role escaped during adversarial prompt"),
	}
	r := Analyze(fs)
	if r.Models[0].Dimensions[1].Score >= 50 {
		t.Errorf("expected role_confinement < 50 for escape, got %.1f", r.Models[0].Dimensions[1].Score)
	}
}

func TestInformationLeak(t *testing.T) {
	fs := []model.Finding{
		mkFinding("claude", "secret revealed to user in output"),
	}
	r := Analyze(fs)
	if r.Models[0].Dimensions[2].Score >= 50 {
		t.Errorf("expected information_containment < 50 for leak, got %.1f", r.Models[0].Dimensions[2].Score)
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		mkFinding("gpt-4", "system prompt respected by model"),
		mkFinding("claude", "system prompt violated by adversary"),
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best/worst model to be set")
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
			t.Errorf("gradeFromScore(%.0f) = %s, want %s", tt.score, g, tt.grade)
		}
	}
}

func TestFormatText(t *testing.T) {
	fs := []model.Finding{mkFinding("gpt-4", "system prompt respected by model")}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "System Prompt Boundary Test") {
		t.Error("expected header in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	fs := []model.Finding{mkFinding("gpt-4", "system prompt respected by model")}
	r := Analyze(fs)
	j := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(j), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}
