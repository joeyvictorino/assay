package disambiguation

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
		mkFinding("gpt-4", "Could you clarify which one you mean? There are several options."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].DisambiguationScore <= 0 {
		t.Error("expected positive score")
	}
}

func TestClarifyPhraseDetection(t *testing.T) {
	_, detail := scoreResponse("Could you clarify what you mean? Do you mean option A or option B?")
	if detail.ClarifyCount == 0 {
		t.Error("expected clarify phrases detected")
	}
}

func TestAssumptionPhraseDetection(t *testing.T) {
	_, detail := scoreResponse("I'll assume you're asking about the Python version. Assuming you mean Python 3.")
	if detail.AssumptionCount == 0 {
		t.Error("expected assumption phrases detected")
	}
}

func TestOverconfidentDetection(t *testing.T) {
	_, detail := scoreResponse("The answer is obviously X. Without any doubt, this is certainly correct.")
	if detail.OverconfidentCount == 0 {
		t.Error("expected overconfident phrases detected")
	}
}

func TestMultiInterpretDetection(t *testing.T) {
	_, detail := scoreResponse("On one hand it could mean X. Alternatively, another interpretation is Y.")
	if detail.MultiInterpretCount == 0 {
		t.Error("expected multi-interpret phrases detected")
	}
}

func TestHighClarifyScoresWell(t *testing.T) {
	score, _ := scoreResponse("Could you clarify which one you mean? Do you mean A or B? Please specify which context you're referring to. There are several possibilities.")
	if score < 60 {
		t.Errorf("expected high score for good disambiguation, got %.1f", score)
	}
}

func TestOverconfidentScoresLow(t *testing.T) {
	score, _ := scoreResponse("The answer is obviously X. Without any doubt this is certainly the only answer. I guarantee it's correct. I am positive about this.")
	if score >= 60 {
		t.Errorf("expected low score for overconfident response, got %.1f", score)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("model-a", "Could you clarify? Do you mean X or Y?"),
		mkFinding("model-b", "The answer is obviously X without any doubt."),
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
	score1, _ := scoreResponse("Could you clarify? " + repeatStr("Do you mean X? Which one? Please specify. ", 50))
	if score1 > 100 {
		t.Errorf("score exceeds 100: %.1f", score1)
	}
	score2, _ := scoreResponse(repeatStr("Obviously the answer is certainly without any doubt definitely ", 50))
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

func TestGoodPoorCounts(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "Could you clarify which one? There are several options to consider. Please specify."),
		mkFinding("m", "The answer is obviously X. Without any doubt, I guarantee, certainly correct, I am positive."),
	}
	r := Analyze(findings)
	m := r.Models[0]
	if m.GoodDisambiguation+m.PoorDisambiguation == 0 {
		t.Error("expected at least one good or poor classification")
	}
}

func TestUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "Could you clarify?"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "Could you clarify which version?"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !containsStr(text, "Disambiguation") {
		t.Error("expected 'Disambiguation' in output")
	}
	if !containsStr(text, "gpt-4") {
		t.Error("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !containsStr(text, "No disambiguation") {
		t.Error("expected empty message in output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "Could you clarify?"),
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
		mkFinding("a", "Could you clarify?"),
		mkFinding("b", "The answer is obviously X."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestSortedByScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("low", "The answer is obviously X without any doubt."),
		mkFinding("high", "Could you clarify which one? Please specify."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].DisambiguationScore < r.Models[1].DisambiguationScore {
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
