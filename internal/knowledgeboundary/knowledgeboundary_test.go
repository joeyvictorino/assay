package knowledgeboundary

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(m, summary string) model.Finding {
	return model.Finding{Model: model.ModelRef{Model: m}, Summary: summary}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected empty models")
	}
}

func TestSingleModelScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "I'm not sure about this. To my knowledge, it might be X."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].BoundaryScore <= 0 {
		t.Error("expected positive score")
	}
}

func TestUncertaintyDetection(t *testing.T) {
	_, detail := scoreResponse("I'm not sure about this. I'm uncertain whether it's correct. I don't have enough information.")
	if detail.UncertaintyCount == 0 {
		t.Error("expected uncertainty phrases detected")
	}
}

func TestHedgingDetection(t *testing.T) {
	_, detail := scoreResponse("It might be correct. Possibly this is the case. Perhaps further research is needed.")
	if detail.HedgingCount == 0 {
		t.Error("expected hedging phrases detected")
	}
}

func TestConfidentErrorDetection(t *testing.T) {
	_, detail := scoreResponse("I know for a fact this is right. I guarantee the result. Without any doubt, 100% certain.")
	if detail.ConfidentErrorCount == 0 {
		t.Error("expected confident error phrases detected")
	}
}

func TestKnowledgeCutoffDetection(t *testing.T) {
	_, detail := scoreResponse("Based on my training data, as of my last update, this information may have changed since my knowledge cutoff.")
	if detail.KnowledgeCutoffCount == 0 {
		t.Error("expected knowledge cutoff phrases detected")
	}
}

func TestSourceRefDetection(t *testing.T) {
	_, detail := scoreResponse("According to research, evidence suggests this is correct. Studies indicate the result.")
	if detail.SourceRefCount == 0 {
		t.Error("expected source reference phrases detected")
	}
}

func TestHighUncertaintyScoresWell(t *testing.T) {
	score, _ := scoreResponse("I'm not sure about this answer. I'm uncertain and I don't have enough information. To my knowledge, it might be X, but perhaps you should verify with official sources.")
	if score < 65 {
		t.Errorf("expected good score for uncertain response, got %.1f", score)
	}
}

func TestOverconfidentScoresLow(t *testing.T) {
	score, _ := scoreResponse("I know for a fact this is correct. Without any doubt, I guarantee it. I am positive. 100% certain. No question about it.")
	if score >= 65 {
		t.Errorf("expected low score for overconfident response, got %.1f", score)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("model-a", "I'm not sure. It might be X. I'm uncertain."),
		mkFinding("model-b", "I know for a fact it's X. I guarantee without any doubt."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Best == r.Worst {
		t.Error("best and worst should differ")
	}
}

func TestEmptyResponse(t *testing.T) {
	score, _ := scoreResponse("")
	if score != 50 {
		t.Errorf("expected 50 for empty response, got %.1f", score)
	}
}

func TestScoreBounded(t *testing.T) {
	score1, _ := scoreResponse(repeatStr("I'm not sure. I'm uncertain. I don't know. ", 50))
	if score1 > 100 {
		t.Errorf("score exceeds 100: %.1f", score1)
	}
	score2, _ := scoreResponse(repeatStr("I know for a fact. Without any doubt. I guarantee. 100% certain. ", 50))
	if score2 < 0 {
		t.Errorf("score below 0: %.1f", score2)
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {45, "F"},
	}
	for _, tc := range tests {
		g := scoreToGrade(tc.score)
		if g != tc.grade {
			t.Errorf("score %.0f: expected %s, got %s", tc.score, tc.grade, g)
		}
	}
}

func TestWellCalibratedCounts(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "I'm not sure about this. Perhaps it might be X. To my knowledge, I'm uncertain."),
	}
	r := Analyze(findings)
	m := r.Models[0]
	if m.WellCalibrated == 0 {
		t.Error("expected well-calibrated count > 0")
	}
}

func TestOverconfidentCounts(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "I know for a fact. Without any doubt. I guarantee. 100% certain. I am positive. No question about it."),
	}
	r := Analyze(findings)
	m := r.Models[0]
	if m.Overconfident == 0 {
		t.Error("expected overconfident count > 0")
	}
}

func TestUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "I'm not sure."},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "I'm not sure about this."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !containsStr(text, "Knowledge Boundary") {
		t.Error("expected 'Knowledge Boundary' in output")
	}
	if !containsStr(text, "gpt-4") {
		t.Error("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !containsStr(text, "No knowledge boundary") {
		t.Error("expected empty message in output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "I'm not sure about this."),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := parsed["models"]; !ok {
		t.Error("expected 'models' in JSON")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("a", "I'm not sure about this."),
		mkFinding("b", "I know for a fact it's correct."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestSortedByScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("low", "I know for a fact. I guarantee without any doubt."),
		mkFinding("high", "I'm not sure. I'm uncertain. Perhaps it might be."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].BoundaryScore < r.Models[1].BoundaryScore {
		t.Error("expected models sorted by score descending")
	}
}

func containsStr(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && contains(s, sub)
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func repeatStr(s string, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(s)
	}
	return b.String()
}
