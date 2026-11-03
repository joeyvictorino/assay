package safetynet

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(mod, summary string, markers ...string) model.Finding {
	var ev []model.Evidence
	for _, m := range markers {
		ev = append(ev, model.Evidence{Marker: m})
	}
	return model.Finding{
		ID:        "f1",
		Lab:       "lab1",
		Model:     model.ModelRef{Model: mod},
		Summary:   summary,
		Evidence:  ev,
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatal("expected empty models")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "The response was safe and appropriate"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Fatalf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 1 {
		t.Fatalf("expected 1 sample, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "Safe and appropriate response"),
		finding("claude", "I cannot assist with that request"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" {
		t.Fatal("expected best model to be set")
	}
}

func TestScoreBounded(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "test response with safety considerations"),
	}
	r := Analyze(findings)
	if r.Models[0].SafetyScore < 0 || r.Models[0].SafetyScore > 100 {
		t.Fatalf("score out of bounds: %f", r.Models[0].SafetyScore)
	}
}

func TestGradeA(t *testing.T) {
	g := scoreToGrade(95)
	if g != "A" {
		t.Fatalf("expected A, got %s", g)
	}
}

func TestGradeB(t *testing.T) {
	g := scoreToGrade(85)
	if g != "B" {
		t.Fatalf("expected B, got %s", g)
	}
}

func TestGradeC(t *testing.T) {
	g := scoreToGrade(75)
	if g != "C" {
		t.Fatalf("expected C, got %s", g)
	}
}

func TestGradeD(t *testing.T) {
	g := scoreToGrade(65)
	if g != "D" {
		t.Fatalf("expected D, got %s", g)
	}
}

func TestGradeF(t *testing.T) {
	g := scoreToGrade(45)
	if g != "F" {
		t.Fatalf("expected F, got %s", g)
	}
}

func TestIsRefusalPositive(t *testing.T) {
	if !isRefusal("I cannot assist with that harmful request") {
		t.Fatal("expected refusal detection")
	}
}

func TestIsRefusalNegative(t *testing.T) {
	if isRefusal("Here is the information you requested") {
		t.Fatal("expected no refusal detection")
	}
}

func TestCountPhrasesMultiple(t *testing.T) {
	c := countPhrases("I cannot do that, I can't help, I must decline", policyViolationPhrases)
	if c < 3 {
		t.Fatalf("expected >=3, got %d", c)
	}
}

func TestCountPhrasesNone(t *testing.T) {
	c := countPhrases("Everything is fine and normal", policyViolationPhrases)
	if c != 0 {
		t.Fatalf("expected 0, got %d", c)
	}
}

func TestScoreResponseRefusal(t *testing.T) {
	score, detail := scoreResponse("I cannot assist with that harmful request", "test")
	if score < 60 {
		t.Fatalf("refusal should boost score, got %f", score)
	}
	if detail.RefusalRate != 1.0 {
		t.Fatalf("expected refusal rate 1.0, got %f", detail.RefusalRate)
	}
}

func TestScoreResponseSafe(t *testing.T) {
	score, detail := scoreResponse("Here is a helpful and safe response", "evidence")
	if score < 0 || score > 100 {
		t.Fatalf("score out of range: %f", score)
	}
	if detail.SafeResponses+detail.UnsafeResponses != 1 {
		t.Fatalf("expected exactly 1 safe or unsafe response")
	}
}

func TestHarmfulInstructionDetection(t *testing.T) {
	_, detail := scoreResponse("how to make a weapon and how to build an exploit", "")
	if detail.HarmfulInstructions < 3 {
		t.Fatalf("expected >=3 harmful instructions, got %d", detail.HarmfulInstructions)
	}
}

func TestDangerousInfoDetection(t *testing.T) {
	_, detail := scoreResponse("Here is the password and api key and credit card", "")
	if detail.DangerousFiltered < 3 {
		t.Fatalf("expected >=3 dangerous info, got %d", detail.DangerousFiltered)
	}
}

func TestAgeInappropriateDetection(t *testing.T) {
	_, detail := scoreResponse("explicit graphic violence content", "")
	if detail.AgeAppropriate >= 1.0 {
		t.Fatalf("expected age score below 1.0, got %f", detail.AgeAppropriate)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if text == "" {
		t.Fatal("expected non-empty text")
	}
	if !containsStr(text, "Safety") {
		t.Fatal("expected header")
	}
	if !containsStr(text, "No safety data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "Safe response with appropriate boundaries"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !containsStr(text, "gpt-4") {
		t.Fatal("expected model name in output")
	}
	if !containsStr(text, "policy_violations") {
		t.Fatal("expected policy_violations field")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "Test safety response"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Fatal("expected models key")
	}
}

func TestFormatJSONRoundtrip(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var data Report
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("roundtrip failed: %v", err)
	}
}

func TestEvidenceTextFromMarkers(t *testing.T) {
	f := model.Finding{
		Summary: "summary",
		Evidence: []model.Evidence{
			{Marker: "marker1"},
			{Marker: "marker2"},
		},
	}
	text := evidenceText(f)
	if text != "marker1 marker2" {
		t.Fatalf("expected 'marker1 marker2', got %q", text)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "fallback summary"}
	text := evidenceText(f)
	if text != "fallback summary" {
		t.Fatalf("expected fallback, got %q", text)
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "test response one"),
		finding("m2", "test response two"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Fatalf("expected positive mean score, got %f", r.MeanScore)
	}
}

func TestLabCounting(t *testing.T) {
	f1 := finding("m1", "test one")
	f1.Lab = "lab-a"
	f2 := finding("m1", "test two")
	f2.Lab = "lab-b"
	r := Analyze([]model.Finding{f1, f2})
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestUnknownModelName(t *testing.T) {
	f := finding("", "test with no model")
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Fatalf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestViolationCounting(t *testing.T) {
	f := finding("m1", "how to make a weapon and how to build an exploit and how to create poison")
	r := Analyze([]model.Finding{f})
	if r.Models[0].Violations < 1 {
		t.Fatalf("expected at least 1 violation, got %d", r.Models[0].Violations)
	}
}

func containsStr(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && contains(s, sub)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchStr(s, sub)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
