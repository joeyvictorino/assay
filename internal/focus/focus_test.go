package focus

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Summary: summary,
		Lab:     "test-lab",
	}
}

func TestScoreResponseClean(t *testing.T) {
	score, detail := scoreResponse("explain sql injection", "SQL injection is a code injection technique that exploits security vulnerabilities in database queries.", "")
	if score < 70 {
		t.Errorf("focused response should score >= 70, got %.1f", score)
	}
	if detail.TangentCount != 0 {
		t.Errorf("expected no tangents, got %d", detail.TangentCount)
	}
}

func TestScoreResponseWithTangents(t *testing.T) {
	text := "SQL injection is dangerous. Speaking of which, by the way, on a related note, you might want to learn about XSS too. Incidentally, fun fact about databases."
	score, detail := scoreResponse("explain sql injection", text, "")
	if detail.TangentCount == 0 {
		t.Error("expected tangent detections")
	}
	if score >= 85 {
		t.Errorf("tangent-heavy response should score < 85, got %.1f", score)
	}
}

func TestScoreResponseWithUnsolicitedAdvice(t *testing.T) {
	text := "The answer is 42. You might also want to learn calculus. Additionally you could study physics. I would also recommend reading more."
	score, detail := scoreResponse("what is the answer", text, "")
	if detail.UnsolicitedAdvice == 0 {
		t.Error("expected unsolicited advice detections")
	}
	if score >= 85 {
		t.Errorf("advice-heavy response should score < 85, got %.1f", score)
	}
}

func TestScoreResponseWithBoilerplate(t *testing.T) {
	text := "The answer is 42. Hope this helps! Let me know if you have any other questions. Feel free to ask if you need more."
	_, detail := scoreResponse("what is the answer", text, "")
	if detail.BoilerplateCount == 0 {
		t.Error("expected boilerplate detections")
	}
}

func TestScoreResponseWithCaveats(t *testing.T) {
	text := "The answer is 42. However it's worth noting some edge cases. Keep in mind that this varies. Please note that results differ."
	_, detail := scoreResponse("", text, "")
	if detail.CaveatCount == 0 {
		t.Error("expected caveat detections")
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, _ := scoreResponse("", "", "")
	if score < 80 {
		t.Errorf("empty response should get baseline score, got %.1f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{85, "A"},
		{75, "B"},
		{60, "C"},
		{45, "D"},
		{30, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestPromptWordSet(t *testing.T) {
	set := promptWordSet("What is SQL injection vulnerability?")
	if !set["sql"] {
		t.Error("expected 'sql' in word set")
	}
	if !set["injection"] {
		t.Error("expected 'injection' in word set")
	}
	if set["is"] {
		t.Error("stop word 'is' should be excluded")
	}
}

func TestPromptCoverage(t *testing.T) {
	coverage := computePromptCoverage("explain sql injection", "SQL injection allows attackers to manipulate database queries.")
	if coverage < 0.5 {
		t.Errorf("expected decent coverage, got %.2f", coverage)
	}
}

func TestPromptCoverageEmpty(t *testing.T) {
	coverage := computePromptCoverage("", "some response")
	if coverage != 1.0 {
		t.Errorf("empty prompt should give 1.0 coverage, got %.2f", coverage)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("empty findings should produce empty report")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "The function returns 42."),
		makeFinding("gpt-4", "This is a focused answer."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Focused answer."),
		makeFinding("claude", "Speaking of which, by the way, fun fact, on a related note, I would also recommend, hope this helps, let me know if you have any questions."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best/worst model to be set")
	}
}

func TestAnalyzeSortsByScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("unfocused", "Speaking of which, by the way, on a related note, fun fact, you might also want to, hope this helps, let me know if you have any more questions."),
		makeFinding("focused", "Returns 42."),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].FocusScore < r.Models[1].FocusScore {
		t.Error("models should be sorted by score descending")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "test", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No focus data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextHeader(t *testing.T) {
	findings := []model.Finding{makeFinding("gpt-4", "test")}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Focus Analysis") {
		t.Error("expected header")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{makeFinding("gpt-4", "test")}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "focus_score") {
		t.Error("expected focus_score in JSON")
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback",
		Evidence: []model.Evidence{
			{Marker: "m1"},
			{Marker: "m2"},
		},
	}
	got := evidenceText(f)
	if got != "m1 m2" {
		t.Errorf("expected joined markers, got %q", got)
	}
}

func TestLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "a"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab2", Summary: "b"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "c"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}
