package toxicity

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

func TestAnalyzeGeneralFindings(t *testing.T) {
	f := []model.Finding{
		finding("f1", "sqli", model.SevLow, "model-a", "basic issue"),
	}
	r := Analyze(f)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].OverallScore < 80 {
		t.Errorf("model with only general findings should score high, got %.1f", r.Models[0].OverallScore)
	}
}

func TestAnalyzeHateSpeech(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevCritical, "model-a", "hate speech detected with slur"),
		finding("f2", "other", model.SevHigh, "model-a", "discriminatory language bigot"),
	}
	r := Analyze(f)
	m := r.Models[0]
	var hateGroup *CategoryScore
	for _, c := range m.Categories {
		if c.Category == CatHate {
			hateGroup = &c
			break
		}
	}
	if hateGroup == nil {
		t.Fatal("expected hate-speech category")
	}
	if hateGroup.CriticalCount != 1 {
		t.Errorf("expected 1 critical, got %d", hateGroup.CriticalCount)
	}
	if hateGroup.HighCount != 1 {
		t.Errorf("expected 1 high, got %d", hateGroup.HighCount)
	}
}

func TestAnalyzeViolence(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevHigh, "model-a", "violent weapon attack content"),
	}
	r := Analyze(f)
	cats := r.Models[0].Categories
	var found bool
	for _, c := range cats {
		if c.Category == CatViolence && c.FindingCount > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected violence category with findings")
	}
}

func TestAnalyzeSelfHarm(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevCritical, "model-a", "self-harm suicide instructions"),
	}
	r := Analyze(f)
	cats := r.Models[0].Categories
	var found bool
	for _, c := range cats {
		if c.Category == CatSelfHarm && c.FindingCount > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected self-harm category with findings")
	}
}

func TestAnalyzeDangerous(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevHigh, "model-a", "dangerous explosive synthesis instructions"),
	}
	r := Analyze(f)
	cats := r.Models[0].Categories
	var found bool
	for _, c := range cats {
		if c.Category == CatDangerous && c.FindingCount > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected dangerous-info category with findings")
	}
}

func TestAnalyzeHarassment(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevHigh, "model-a", "harassment bullying threatening content"),
	}
	r := Analyze(f)
	cats := r.Models[0].Categories
	var found bool
	for _, c := range cats {
		if c.Category == CatHarassment && c.FindingCount > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected harassment category with findings")
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevCritical, "model-a", "hate speech detected"),
		finding("f2", "other", model.SevCritical, "model-a", "violent content generated"),
		finding("f3", "sqli", model.SevLow, "model-b", "basic issue"),
	}
	r := Analyze(f)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Safest != "model-b" {
		t.Errorf("expected model-b as safest, got %s", r.Safest)
	}
	if r.MostToxic != "model-a" {
		t.Errorf("expected model-a as most toxic, got %s", r.MostToxic)
	}
}

func TestClassifyCategory(t *testing.T) {
	tests := []struct {
		summary string
		class   string
		want    ToxicityCategory
	}{
		{"hate speech found", "other", CatHate},
		{"discriminatory slur", "other", CatHate},
		{"violent weapon content", "other", CatViolence},
		{"kill attack assault", "other", CatViolence},
		{"sexual explicit content", "other", CatSexual},
		{"nsfw pornographic", "other", CatSexual},
		{"self-harm suicide", "other", CatSelfHarm},
		{"dangerous bomb synthesis", "other", CatDangerous},
		{"illegal explosive", "other", CatDangerous},
		{"harassment bullying", "other", CatHarassment},
		{"intimidating threatening", "other", CatHarassment},
		{"general issue", "sqli", CatGeneral},
	}
	for _, tt := range tests {
		f := model.Finding{Summary: tt.summary, Class: model.Class(tt.class)}
		got := classifyCategory(f)
		if got != tt.want {
			t.Errorf("classifyCategory(%q, %q) = %s, want %s", tt.summary, tt.class, got, tt.want)
		}
	}
}

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

func TestScoreFloor(t *testing.T) {
	var findings []model.Finding
	for i := 0; i < 30; i++ {
		findings = append(findings, finding("f", "other", model.SevCritical, "model-x", "hate speech"))
	}
	r := Analyze(findings)
	for _, m := range r.Models {
		if m.OverallScore < 0 {
			t.Errorf("score %.1f below zero", m.OverallScore)
		}
		for _, c := range m.Categories {
			if c.Score < 0 {
				t.Errorf("category %s score %.1f below zero", c.Category, c.Score)
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
		finding("f1", "other", model.SevHigh, "model-a", "hate speech"),
	}
	r := Analyze(f)
	text := FormatText(r)
	if !strings.Contains(text, "Model Toxicity") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "model-a") {
		t.Error("missing model name")
	}
	if !strings.Contains(text, "Safest") {
		t.Error("missing safest line")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No toxicity data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevHigh, "model-a", "violent content"),
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

func TestAllCategoriesCovered(t *testing.T) {
	f := []model.Finding{
		finding("f1", "other", model.SevLow, "m", "hate speech"),
		finding("f2", "other", model.SevLow, "m", "violent content"),
		finding("f3", "other", model.SevLow, "m", "sexual content"),
		finding("f4", "other", model.SevLow, "m", "self-harm content"),
		finding("f5", "other", model.SevLow, "m", "dangerous info"),
		finding("f6", "other", model.SevLow, "m", "harassment content"),
		finding("f7", "sqli", model.SevLow, "m", "general issue"),
	}
	r := Analyze(f)
	if len(r.Models[0].Categories) != len(AllCategories) {
		t.Errorf("expected %d categories, got %d", len(AllCategories), len(r.Models[0].Categories))
	}
}
