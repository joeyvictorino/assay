package rhetorical

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
	findings := []model.Finding{mkFinding("gpt-4", "This is a test")}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
}

func TestLogosPhrasesDetected(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "Because the evidence shows that data indicates the analysis shows results")}
	r := Analyze(findings)
	if r.Models[0].Detail.LogosCount == 0 {
		t.Fatal("expected logos phrases to be counted")
	}
}

func TestEthosPhrasesDetected(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "According to experts, studies show this is best practice and industry standard")}
	r := Analyze(findings)
	if r.Models[0].Detail.EthosCount == 0 {
		t.Fatal("expected ethos phrases to be counted")
	}
}

func TestPathosPhrasesDetected(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "Imagine the consequences. Consider the impact on users. Critically, the risk is significant.")}
	r := Analyze(findings)
	if r.Models[0].Detail.PathosCount == 0 {
		t.Fatal("expected pathos phrases to be counted")
	}
}

func TestStrongRhetoricScoresHigher(t *testing.T) {
	strong := mkFinding("strong", "Because the evidence shows, according to experts, first we must consider, specifically for example")
	weak := mkFinding("weak", "sort of kind of maybe i guess whatever or something not sure unclear")
	r := Analyze([]model.Finding{strong, weak})
	var strongScore, weakScore float64
	for _, m := range r.Models {
		if m.Model == "strong" {
			strongScore = m.RhetoricalScore
		}
		if m.Model == "weak" {
			weakScore = m.RhetoricalScore
		}
	}
	if strongScore <= weakScore {
		t.Fatalf("strong (%.1f) should score higher than weak (%.1f)", strongScore, weakScore)
	}
}

func TestBestWorstModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("good", "Therefore because consequently the evidence shows according to experts first second"),
		mkFinding("bad", "sort of kind of maybe i guess whatever unclear not sure"),
	}
	r := Analyze(findings)
	if r.BestModel == "" || r.WorstModel == "" {
		t.Fatal("expected best and worst to be set")
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
		mkFinding("m1", "One"),
		mkFinding("m1", "Two"),
		mkFinding("m1", "Three"),
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
	if scoreToGrade(95) != "A" {
		t.Fatal("expected A")
	}
}

func TestGradeB(t *testing.T) {
	if scoreToGrade(85) != "B" {
		t.Fatal("expected B")
	}
}

func TestGradeC(t *testing.T) {
	if scoreToGrade(75) != "C" {
		t.Fatal("expected C")
	}
}

func TestGradeD(t *testing.T) {
	if scoreToGrade(65) != "D" {
		t.Fatal("expected D")
	}
}

func TestGradeF(t *testing.T) {
	if scoreToGrade(50) != "F" {
		t.Fatal("expected F")
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
	if !strings.Contains(text, "No rhetorical data") {
		t.Fatal("expected no data message")
	}
}

func TestFormatTextWithData(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "Because therefore")}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Rhetorical") {
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

func TestConcernsForWeakRhetoric(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "just a plain response with no rhetorical devices at all"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Fatal("expected concerns for response lacking logos and ethos")
	}
}

func TestEvidenceMarkerUsed(t *testing.T) {
	f := model.Finding{
		Model:   model.ModelRef{Model: "m1"},
		Summary: "Test",
		Lab:     "lab1",
		Evidence: []model.Evidence{
			{Marker: "because therefore consequently the evidence shows"},
		},
	}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Detail.LogosCount == 0 {
		t.Fatal("expected evidence markers to be included in analysis")
	}
}

func TestModelsAreSorted(t *testing.T) {
	findings := []model.Finding{
		mkFinding("low", "sort of kind of maybe whatever"),
		mkFinding("high", "Because therefore the evidence shows according to experts first second specifically"),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].RhetoricalScore < r.Models[1].RhetoricalScore {
		t.Fatal("models should be sorted descending by score")
	}
}

func TestStructureDetected(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "First, we do this. Second, we consider. Finally, in conclusion, the key points are clear.")}
	r := Analyze(findings)
	if r.Models[0].Detail.StructureScore <= 0.2 {
		t.Fatal("expected structure score above baseline")
	}
}

func TestClarityDetected(t *testing.T) {
	findings := []model.Finding{mkFinding("m1", "Specifically, for example, to clarify, that is to say, namely, for instance")}
	r := Analyze(findings)
	if r.Models[0].Detail.ClarityScore <= 0.3 {
		t.Fatal("expected clarity score above baseline")
	}
}
