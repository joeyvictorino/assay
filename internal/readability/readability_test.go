package readability

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     "test-lab",
		Summary: summary,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestClearResponse(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "First, the system checks input. However, the validation fails. Therefore, the error is returned. In summary, the process is straightforward."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].ReadabilityScore < 70 {
		t.Errorf("clear response should score >= 70, got %.1f", r.Models[0].ReadabilityScore)
	}
}

func TestJargonHeavy(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "We need to leverage synergy to operationalize the paradigm. The holistic ideation drives disruptive scalable actionable deep dive thought leader pivot."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.JargonCount == 0 {
		t.Error("expected jargon detected")
	}
	if r.Models[0].ReadabilityScore >= 80 {
		t.Errorf("jargon-heavy response should score < 80, got %.1f", r.Models[0].ReadabilityScore)
	}
}

func TestPassiveVoice(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "The task was done by the agent. The result was found to be correct. The operation was completed and was given to the user. It was made available."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.PassiveCount == 0 {
		t.Error("expected passive voice detected")
	}
}

func TestTransitions(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "First, check the input. However, the data may be invalid. Therefore, validate before processing. In addition, log the errors. In conclusion, this approach works."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.TransitionCues == 0 {
		t.Error("expected transition cues detected")
	}
}

func TestStructure(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "First, review the code. Second, run the tests. Third, deploy. Finally, monitor the system. The following steps apply."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.StructureCues == 0 {
		t.Error("expected structure cues detected")
	}
}

func TestLongSentences(t *testing.T) {
	long := strings.Repeat("word ", 35) + "end."
	findings := []model.Finding{
		makeFinding("gpt-4", long),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.LongSentences == 0 {
		t.Error("expected long sentences detected")
	}
}

func TestShortSentences(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Yes. No. Ok. Done."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.ShortSentences == 0 {
		t.Error("expected short sentences detected")
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Clear and well-structured response with transitions. However, more detail needed."),
		makeFinding("claude", "We leverage synergy to operationalize the paradigm and utilize holistic ideation for disruptive scalable actionable results."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestGradeA(t *testing.T) {
	if g := scoreToGrade(95); g != "A" {
		t.Errorf("expected A, got %s", g)
	}
}

func TestGradeB(t *testing.T) {
	if g := scoreToGrade(85); g != "B" {
		t.Errorf("expected B, got %s", g)
	}
}

func TestGradeC(t *testing.T) {
	if g := scoreToGrade(75); g != "C" {
		t.Errorf("expected C, got %s", g)
	}
}

func TestGradeD(t *testing.T) {
	if g := scoreToGrade(65); g != "D" {
		t.Errorf("expected D, got %s", g)
	}
}

func TestGradeF(t *testing.T) {
	if g := scoreToGrade(45); g != "F" {
		t.Errorf("expected F, got %s", g)
	}
}

func TestScoreBounded(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Word."),
	}
	r := Analyze(findings)
	if r.Models[0].ReadabilityScore < 0 || r.Models[0].ReadabilityScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].ReadabilityScore)
	}
}

func TestEmptyText(t *testing.T) {
	score, _ := scoreResponse("")
	if score != 100 {
		t.Errorf("empty text should score 100, got %.1f", score)
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "A simple clear response."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Readability Analysis") {
		t.Error("expected header in text output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No readability data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "A well-written response. However, it could be improved."),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	if !strings.Contains(out, "readability_score") {
		t.Error("expected readability_score in JSON")
	}
}

func TestMostLeastReadable(t *testing.T) {
	findings := []model.Finding{
		makeFinding("good-model", "First, check input. However, validate data. Therefore, this works. In summary, clear steps."),
		makeFinding("bad-model", "We leverage synergy to operationalize paradigm utilize holistic ideation disruptive scalable actionable deep dive thought leader."),
	}
	r := Analyze(findings)
	if r.MostReadable == "" || r.LeastReadable == "" {
		t.Error("expected most/least readable set")
	}
	if r.MostReadable == r.LeastReadable {
		t.Error("most and least readable should differ")
	}
}

func TestAvgWordLength(t *testing.T) {
	avg := avgWordLength("the cat sat")
	if avg < 2 || avg > 4 {
		t.Errorf("expected avg word length 2-4, got %.1f", avg)
	}
}

func TestSplitSentences(t *testing.T) {
	sents := splitSentences("Hello world. How are you? Fine!")
	if len(sents) != 3 {
		t.Errorf("expected 3 sentences, got %d", len(sents))
	}
}
