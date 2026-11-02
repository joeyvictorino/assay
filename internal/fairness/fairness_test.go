package fairness

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(id string, class model.Class, sev model.Severity, modelName, summary string) model.Finding {
	return model.Finding{
		ID:       id,
		Class:    class,
		Severity: sev,
		Model:    model.ModelRef{Model: modelName},
		Summary:  summary,
		State:    model.StateValidated,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
	if r.MeanScore != 0 {
		t.Fatalf("expected 0 mean, got %.1f", r.MeanScore)
	}
}

func TestAnalyzeNoGroupFindings(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevLow, "model-a", "basic sql issue"),
	}
	r := Analyze(f)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].OverallScore < 80 {
		t.Errorf("model with only general findings should score high, got %.1f", r.Models[0].OverallScore)
	}
}

func TestAnalyzeGenderBias(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevCritical, "model-a", "gender bias in response"),
		finding("f2", "other", model.SevHigh, "model-a", "male stereotyping detected"),
	}
	r := Analyze(f)
	m := r.Models[0]
	var genderGroup *GroupScore
	for _, g := range m.Groups {
		if g.Group == GroupGender {
			genderGroup = &g
			break
		}
	}
	if genderGroup == nil {
		t.Fatal("expected gender group")
	}
	if genderGroup.CriticalCount != 1 {
		t.Errorf("expected 1 critical, got %d", genderGroup.CriticalCount)
	}
	if genderGroup.HighCount != 1 {
		t.Errorf("expected 1 high, got %d", genderGroup.HighCount)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevCritical, "model-a", "gender bias in output"),
		finding("f2", "other", model.SevCritical, "model-a", "racial stereotyping"),
		finding("f3", "sqli", model.SevLow, "model-b", "basic issue"),
	}
	r := Analyze(f)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Fairest != "model-b" {
		t.Errorf("expected model-b as fairest, got %s", r.Fairest)
	}
	if r.LeastFair != "model-a" {
		t.Errorf("expected model-a as least fair, got %s", r.LeastFair)
	}
}

func TestClassifyGroup(t *testing.T) {
	tests := []struct {
		summary string
		class   string
		want    DemographicGroup
	}{
		{"gender bias found", "other", GroupGender},
		{"female stereotyping", "other", GroupGender},
		{"age discrimination", "other", GroupAge},
		{"elder treatment", "other", GroupAge},
		{"ethnic slur detected", "other", GroupEthnicity},
		{"racial profiling", "other", GroupRace},
		{"language barrier", "other", GroupLanguage},
		{"locale mismatch", "other", GroupLanguage},
		{"general issue", "sqli", GroupGeneral},
	}
	for _, tt := range tests {
		f := model.Finding{Summary: tt.summary, Class: model.Class(tt.class)}
		got := classifyGroup(f)
		expected := tt.want
		if tt.want == GroupRace {
			expected = GroupEthnicity
		}
		if got != expected {
			t.Errorf("classifyGroup(%q, %q) = %s, want %s", tt.summary, tt.class, got, expected)
		}
	}
}

// GroupRace alias for test readability — "racial" maps to ethnicity.
const GroupRace = GroupEthnicity

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{100, "A"}, {90, "A"}, {89, "B"}, {80, "B"},
		{79, "C"}, {70, "C"}, {69, "D"}, {60, "D"},
		{59, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestDisparityPenalty(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevCritical, "model-a", "gender bias critical"),
		finding("f2", "other", model.SevCritical, "model-a", "gender bias critical 2"),
		finding("f3", "other", model.SevCritical, "model-a", "gender bias critical 3"),
	}
	r := Analyze(f)
	m := r.Models[0]
	if m.Disparity == 0 {
		t.Error("expected non-zero disparity with concentrated findings")
	}
}

func TestScoreFloor(t *testing.T) {
	var findings []model.Finding
	for i := 0; i < 30; i++ {
		findings = append(findings, finding("f", "other", model.SevCritical, "model-x", "gender bias"))
	}
	r := Analyze(findings)
	for _, m := range r.Models {
		if m.OverallScore < 0 {
			t.Errorf("score %.1f below zero", m.OverallScore)
		}
		for _, g := range m.Groups {
			if g.Score < 0 {
				t.Errorf("group %s score %.1f below zero", g.Group, g.Score)
			}
		}
	}
}

func TestUnknownModel(t *testing.T) {
	f := []model.Finding{
		{ID: "f1", Class: "other", Severity: model.SevLow, State: model.StateValidated, Summary: "test"},
	}
	r := Analyze(f)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %q", r.Models[0].Model)
	}
}

func TestFormatTextOutput(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevHigh, "model-a", "gender issue"),
	}
	r := Analyze(f)
	text := FormatText(r)
	if !strings.Contains(text, "Model Fairness") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "model-a") {
		t.Error("missing model name")
	}
	if !strings.Contains(text, "Fairest") {
		t.Error("missing fairest line")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No fairness data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevHigh, "model-a", "age discrimination"),
	}
	r := Analyze(f)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(parsed.Models))
	}
}

func TestAllGroupsCovered(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevLow, "m", "gender issue"),
		finding("f2", "other", model.SevLow, "m", "age issue"),
		finding("f3", "other", model.SevLow, "m", "ethnic issue"),
		finding("f4", "other", model.SevLow, "m", "language issue"),
		finding("f5", "sqli", model.SevLow, "m", "general issue"),
	}
	r := Analyze(f)
	if len(r.Models[0].Groups) != len(AllGroups) {
		t.Errorf("expected %d groups, got %d", len(AllGroups), len(r.Models[0].Groups))
	}
}
