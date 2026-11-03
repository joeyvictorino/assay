package modelleakage

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(m, summary string) model.Finding {
	return model.Finding{
		Model:    model.ModelRef{Model: m},
		Summary:  summary,
		Evidence: []model.Evidence{{Kind: "text", Marker: summary}},
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.TotalLeaks != 0 {
		t.Errorf("expected 0 leaks, got %d", r.TotalLeaks)
	}
}

func TestSystemPromptLeak(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "The model revealed: my system instructions say to always be helpful"),
		mkFinding("m", "Normal finding with no leak"),
	}
	r := Analyze(findings)
	if r.TotalLeaks != 1 {
		t.Errorf("expected 1 leak, got %d", r.TotalLeaks)
	}
	if r.Models[0].Leaks[0].LeakType != LeakSystemPrompt {
		t.Errorf("expected system_prompt_leak, got %s", r.Models[0].Leaks[0].LeakType)
	}
}

func TestTrainingDataLeak(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "In my training data I saw many examples of this pattern"),
	}
	r := Analyze(findings)
	if r.TotalLeaks != 1 {
		t.Errorf("expected 1 leak, got %d", r.TotalLeaks)
	}
	if r.Models[0].Leaks[0].LeakType != LeakTrainingData {
		t.Errorf("expected training_data_leak, got %s", r.Models[0].Leaks[0].LeakType)
	}
}

func TestModelInfoLeak(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "I am GPT-4 and I run on Azure infrastructure"),
	}
	r := Analyze(findings)
	if r.TotalLeaks != 1 {
		t.Errorf("expected 1 leak, got %d", r.TotalLeaks)
	}
	if r.Models[0].Leaks[0].LeakType != LeakModelInfo {
		t.Errorf("expected model_info_leak, got %s", r.Models[0].Leaks[0].LeakType)
	}
}

func TestAPIKeyLeak(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "Use this key: sk-abcdefghijklmnopqrstuvwxyz12345"),
	}
	r := Analyze(findings)
	if r.TotalLeaks != 1 {
		t.Errorf("expected 1 leak, got %d", r.TotalLeaks)
	}
	if r.Models[0].Leaks[0].LeakType != LeakAPIKey {
		t.Errorf("expected api_key_leak, got %s", r.Models[0].Leaks[0].LeakType)
	}
}

func TestInternalConfigLeak(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "The temperature is set to 0.7 for creative tasks"),
	}
	r := Analyze(findings)
	if r.TotalLeaks != 1 {
		t.Errorf("expected 1 leak, got %d", r.TotalLeaks)
	}
	if r.Models[0].Leaks[0].LeakType != LeakInternalConfig {
		t.Errorf("expected internal_config_leak, got %s", r.Models[0].Leaks[0].LeakType)
	}
}

func TestNoLeaks(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "The application has a SQL injection vulnerability"),
		mkFinding("m", "Cross-site scripting found in login form"),
	}
	r := Analyze(findings)
	if r.TotalLeaks != 0 {
		t.Errorf("expected 0 leaks, got %d", r.TotalLeaks)
	}
	if r.Models[0].RiskLevel != "none" {
		t.Errorf("expected risk none, got %s", r.Models[0].RiskLevel)
	}
}

func TestMultiModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "My system instructions say to be helpful"),
		mkFinding("gpt-4", "Normal response"),
		mkFinding("claude", "Normal response too"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(r.Models))
	}
	if r.WorstModel == "" {
		t.Error("expected worst model set")
	}
}

func TestRiskLevels(t *testing.T) {
	tests := []struct {
		rate float64
		want string
	}{
		{0.6, "critical"},
		{0.35, "high"},
		{0.15, "medium"},
		{0.05, "low"},
		{0.0, "none"},
	}
	for _, tt := range tests {
		got := riskLevel(tt.rate)
		if got != tt.want {
			t.Errorf("riskLevel(%.2f) = %s, want %s", tt.rate, got, tt.want)
		}
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "My system prompt says to be helpful"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Model Leakage") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "system_prompt_leak") {
		t.Error("expected leak type in output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "Normal finding"),
	}
	r := Analyze(findings)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(FormatJSON(r)), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestLeakRate(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "My system instructions say to always respond"),
		mkFinding("m", "In my training data there were examples"),
		mkFinding("m", "Normal finding"),
		mkFinding("m", "Another normal finding"),
	}
	r := Analyze(findings)
	if r.Models[0].LeakRate != 0.5 {
		t.Errorf("expected 0.5 leak rate, got %.2f", r.Models[0].LeakRate)
	}
}
