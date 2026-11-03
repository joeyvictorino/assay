package tonality

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Summary: summary,
		Lab:     "test-lab",
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatal("expected empty models for nil findings")
	}
}

func TestAnalyzeSingleFinding(t *testing.T) {
	findings := []model.Finding{mkFinding("gpt-4", "This is a test response")}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Fatalf("expected gpt-4, got %s", r.Models[0].Model)
	}
}

func TestFormalPhrasesDetected(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "Therefore, furthermore, in conclusion, this is the result")}
	r := Analyze(findings)
	if r.Models[0].Detail.FormalCount == 0 {
		t.Fatal("expected formal phrases to be counted")
	}
}

func TestCasualPhrasesDetected(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "Hey, gonna kinda just do stuff, you know")}
	r := Analyze(findings)
	if r.Models[0].Detail.CasualCount == 0 {
		t.Fatal("expected casual phrases to be counted")
	}
}

func TestEmpatheticPhrasesDetected(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "I understand your concern. Great question. Happy to help.")}
	r := Analyze(findings)
	if r.Models[0].Detail.EmpatheticCount == 0 {
		t.Fatal("expected empathetic phrases to be counted")
	}
}

func TestHighFormalityScoresWell(t *testing.T) {
	formal := mkFinding("formal", "Therefore, consequently, furthermore, moreover, it should be noted that accordingly this is hereby presented")
	casual := mkFinding("casual", "Hey gonna kinda just do stuff you know pretty much cool awesome basically")
	r := Analyze([]model.Finding{formal, casual})
	var formalScore, casualScore float64
	for _, m := range r.Models {
		if m.Model == "formal" {
			formalScore = m.TonalityScore
		}
		if m.Model == "casual" {
			casualScore = m.TonalityScore
		}
	}
	if formalScore <= casualScore {
		t.Fatalf("formal (%.1f) should score higher than casual (%.1f)", formalScore, casualScore)
	}
}

func TestBestWorstModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("good", "Therefore, furthermore, in conclusion, it should be noted"),
		mkFinding("bad", "Hey gonna kinda just do stuff you know"),
	}
	r := Analyze(findings)
	if r.BestModel == "" {
		t.Fatal("expected best model to be set")
	}
	if r.WorstModel == "" {
		t.Fatal("expected worst model to be set")
	}
	if r.BestModel == r.WorstModel {
		t.Fatal("best and worst should differ")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("a", "Test response one"),
		mkFinding("b", "Test response two"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Fatal("expected positive mean score")
	}
}

func TestUnknownModel(t *testing.T) {
	f := model.Finding{Summary: "test", Lab: "lab1"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Fatalf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestMultipleFindings(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "Response one"),
		mkFinding("m1", "Response two"),
		mkFinding("m1", "Response three"),
	}
	r := Analyze(findings)
	if r.Models[0].SampleCount != 3 {
		t.Fatalf("expected 3 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestLabCount(t *testing.T) {
	f1 := model.Finding{Model: model.ModelRef{Model: "m1"}, Summary: "test1", Lab: "lab-a"}
	f2 := model.Finding{Model: model.ModelRef{Model: "m1"}, Summary: "test2", Lab: "lab-b"}
	r := Analyze([]model.Finding{f1, f2})
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
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
	g := scoreToGrade(50)
	if g != "F" {
		t.Fatalf("expected F, got %s", g)
	}
}

func TestScoreResponseBounds(t *testing.T) {
	score, _ := scoreResponse("test", "test")
	if score < 0 || score > 100 {
		t.Fatalf("score out of bounds: %.1f", score)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No tonality data") {
		t.Fatal("expected no data message")
	}
}

func TestFormatTextWithData(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "Therefore this is formal")}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Tonality") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "m1") {
		t.Fatal("expected model name")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "Test")}
	r := Analyze(findings)
	j := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(j), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestConcernsCountedForCasual(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "Hey gonna kinda just do stuff you know pretty much cool awesome basically honestly like things wanna no worries"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Fatal("expected concerns for highly casual response")
	}
}

func TestEvidenceMarkerUsed(t *testing.T) {
	f := model.Finding{
		Model:   model.ModelRef{Model: "m1"},
		Summary: "Test",
		Lab:     "lab1",
		Evidence: []model.Evidence{
			{Marker: "therefore furthermore consequently"},
		},
	}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Detail.FormalCount == 0 {
		t.Fatal("expected evidence markers to be included in analysis")
	}
}

func TestModelsAreSorted(t *testing.T) {
	findings := []model.Finding{
		mkFinding("low", "hey gonna kinda stuff"),
		mkFinding("high", "Therefore, consequently, furthermore, it should be noted that accordingly"),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].TonalityScore < r.Models[1].TonalityScore {
		t.Fatal("models should be sorted descending by score")
	}
}
