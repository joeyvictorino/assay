package logiccheck

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

func TestSoundReasoning(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Given the premises, therefore it follows that logically the evidence shows this implies a valid inference. Thus the conclusion is sound."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].LogicScore < 70 {
		t.Errorf("sound reasoning should score >= 70, got %.1f", r.Models[0].LogicScore)
	}
}

func TestContradictions(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "This contradicts the previous statement. It conflicts with the previous claim. The argument negates the earlier point and is inconsistent with the data."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.ContradictionCues == 0 {
		t.Error("expected contradictions detected")
	}
}

func TestTautologies(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "This is true by definition. It goes without saying this is a tautological self-evident truth that is always true by nature."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.TautologyCues == 0 {
		t.Error("expected tautologies detected")
	}
}

func TestFallacies(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "This is a straw man argument with ad hominem attacks. It uses a red herring and false dichotomy plus slippery slope reasoning."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.FallacyCues == 0 {
		t.Error("expected fallacies detected")
	}
}

func TestCircularReasoning(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "The circular reasoning here proves its own conclusion. This circular argument assumes what it proves and presupposes the conclusion."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.CircularCues == 0 {
		t.Error("expected circular reasoning detected")
	}
}

func TestNonSequitur(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "This is a non sequitur that does not follow. The unrelated conclusion shows a leap of logic with no logical connection."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.NonSequiturCues == 0 {
		t.Error("expected non sequiturs detected")
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Therefore it follows that logically the evidence shows a valid inference."),
		makeFinding("claude", "This contradicts itself. The circular reasoning assumes what it proves. A straw man argument."),
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
	if r.Models[0].LogicScore < 0 || r.Models[0].LogicScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].LogicScore)
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
		makeFinding("gpt-4", "Therefore the logic is sound."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Logic Check Analysis") {
		t.Error("expected header in text output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No logic data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Therefore this implies a valid argument."),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	if !strings.Contains(out, "logic_score") {
		t.Error("expected logic_score in JSON")
	}
}

func TestBestWorstLogic(t *testing.T) {
	findings := []model.Finding{
		makeFinding("good-model", "Therefore it follows that logically the evidence shows a valid inference based on this reasoning."),
		makeFinding("bad-model", "This contradicts itself with circular reasoning. It assumes what it proves. A straw man with ad hominem. Non sequitur that does not follow."),
	}
	r := Analyze(findings)
	if r.BestLogic == "" || r.WorstLogic == "" {
		t.Error("expected best/worst logic set")
	}
	if r.BestLogic == r.WorstLogic {
		t.Error("best and worst logic should differ")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("m1", "Simple response."),
		makeFinding("m2", "Another response."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Errorf("expected positive mean score, got %.1f", r.MeanScore)
	}
}
