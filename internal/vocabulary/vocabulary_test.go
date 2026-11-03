package vocabulary

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

func TestDiversityPhrasesDetected(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "This shows diverse vocabulary with varied terminology and rich vocabulary")}
	r := Analyze(findings)
	if r.Models[0].Detail.DiversityCount == 0 {
		t.Fatal("expected diversity phrases to be counted")
	}
}

func TestPrecisionPhrasesDetected(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "Using precise terminology with domain-specific and correct usage")}
	r := Analyze(findings)
	if r.Models[0].Detail.PrecisionCount == 0 {
		t.Fatal("expected precision phrases to be counted")
	}
}

func TestRedundancyPhrasesDetected(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "This is redundant and repetitive and verbose with superfluous padding and filler words")}
	r := Analyze(findings)
	if r.Models[0].Detail.RedundancyCount == 0 {
		t.Fatal("expected redundancy phrases to be counted")
	}
}

func TestHighDiversityScoresWell(t *testing.T) {
	diverse := mkFinding("diverse", "Diverse vocabulary with varied terminology, rich vocabulary, lexical variety, synonym usage, alternative phrasing, expressive range")
	redundant := mkFinding("redundant", "Redundant repetitive verbose unnecessarily wordy tautological pleonastic superfluous padding filler words bloated")
	r := Analyze([]model.Finding{diverse, redundant})
	var diverseScore, redundantScore float64
	for _, m := range r.Models {
		if m.Model == "diverse" {
			diverseScore = m.VocabularyScore
		}
		if m.Model == "redundant" {
			redundantScore = m.VocabularyScore
		}
	}
	if diverseScore <= redundantScore {
		t.Fatalf("diverse (%.1f) should score higher than redundant (%.1f)", diverseScore, redundantScore)
	}
}

func TestBestWorstModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("good", "Diverse vocabulary with precise terminology and rich vocabulary"),
		mkFinding("bad", "Redundant repetitive verbose tautological pleonastic superfluous padding"),
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
	if !strings.Contains(text, "No vocabulary data") {
		t.Fatal("expected no data message")
	}
}

func TestFormatTextWithData(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "Diverse vocabulary with precise terminology")}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Vocabulary") {
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

func TestConcernsCountedForRedundant(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "Redundant repetitive verbose unnecessarily wordy tautological pleonastic superfluous padding filler words bloated overwrought circumlocution needlessly complex"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Fatal("expected concerns for highly redundant response")
	}
}

func TestEvidenceMarkerUsed(t *testing.T) {
	f := model.Finding{
		Model:   model.ModelRef{Model: "m1"},
		Summary: "Test",
		Lab:     "lab1",
		Evidence: []model.Evidence{
			{Marker: "diverse vocabulary varied terminology rich vocabulary"},
		},
	}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Detail.DiversityCount == 0 {
		t.Fatal("expected evidence markers to be included in analysis")
	}
}

func TestModelsAreSorted(t *testing.T) {
	findings := []model.Finding{
		mkFinding("low", "redundant repetitive verbose padding filler words"),
		mkFinding("high", "Diverse vocabulary with precise terminology and rich vocabulary lexical variety synonym usage"),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].VocabularyScore < r.Models[1].VocabularyScore {
		t.Fatal("models should be sorted descending by score")
	}
}
