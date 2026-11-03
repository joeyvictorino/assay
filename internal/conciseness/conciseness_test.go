package conciseness

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(mdl, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mdl},
		Summary: summary,
		Lab:     "test-lab",
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatal("expected no models")
	}
}

func TestAnalyzeSingleClean(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "The answer is 42.")}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].ConcisenessScore < 70 {
		t.Errorf("clean response should score well, got %.1f", r.Models[0].ConcisenessScore)
	}
}

func TestAnalyzeFillersLowerScore(t *testing.T) {
	clean := []model.Finding{mkFinding("m1", "The answer is 42.")}
	wordy := []model.Finding{mkFinding("m1", "Basically essentially actually literally the answer is 42. In fact as a matter of fact it is worth noting it should be mentioned.")}
	rc := Analyze(clean)
	rw := Analyze(wordy)
	if rw.Models[0].ConcisenessScore >= rc.Models[0].ConcisenessScore {
		t.Error("wordy should score lower than clean")
	}
}

func TestAnalyzeRedundancyPenalty(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Each and every first and foremost past history future plans end result basic fundamentals close proximity.")}
	r := Analyze(fs)
	if r.Models[0].Detail.RedundantCount == 0 {
		t.Error("expected redundant count > 0")
	}
}

func TestAnalyzePaddingDetected(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "It goes without saying that at the end of the day for all intents and purposes the fact of the matter is clear.")}
	r := Analyze(fs)
	if r.Models[0].Detail.RedundantCount == 0 {
		t.Error("expected padding detected")
	}
}

func TestAnalyzeDirectness(t *testing.T) {
	indirect := []model.Finding{mkFinding("m1", "I would like to suggest it might be possible perhaps we could consider one might argue it could be said that.")}
	r := Analyze(indirect)
	if r.Models[0].Detail.Directness >= 0.85 {
		t.Errorf("indirect phrasing should lower directness, got %.2f", r.Models[0].Detail.Directness)
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "Short clear answer."),
		mkFinding("m2", "Basically essentially in fact as a matter of fact the answer is clear. It is worth noting it should be mentioned it is important to note."),
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best/worst models set")
	}
}

func TestGradeA(t *testing.T) {
	g := scoreToGrade(95)
	if g != "A" {
		t.Errorf("expected A, got %s", g)
	}
}

func TestGradeB(t *testing.T) {
	g := scoreToGrade(85)
	if g != "B" {
		t.Errorf("expected B, got %s", g)
	}
}

func TestGradeC(t *testing.T) {
	g := scoreToGrade(75)
	if g != "C" {
		t.Errorf("expected C, got %s", g)
	}
}

func TestGradeD(t *testing.T) {
	g := scoreToGrade(65)
	if g != "D" {
		t.Errorf("expected D, got %s", g)
	}
}

func TestGradeF(t *testing.T) {
	g := scoreToGrade(45)
	if g != "F" {
		t.Errorf("expected F, got %s", g)
	}
}

func TestScoreBounded(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", strings.Repeat("basically essentially actually literally in fact ", 50))}
	r := Analyze(fs)
	if r.Models[0].ConcisenessScore < 0 || r.Models[0].ConcisenessScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].ConcisenessScore)
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Clean response.")}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Conciseness") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "m1") {
		t.Error("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No conciseness data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Test response.")}
	r := Analyze(fs)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Error("expected models key in JSON")
	}
}

func TestFormatJSONRoundtrip(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestConcernsFlagged(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Basically essentially actually literally in fact as a matter of fact to be honest in my opinion each and every first and foremost.")}
	r := Analyze(fs)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns flagged for heavy filler use")
	}
}

func TestVerbosePhrasesPenalize(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "It is clear that it is evident that it is apparent that it is obvious that there is no doubt that it can be seen that.")}
	r := Analyze(fs)
	if r.Models[0].Detail.RedundantCount == 0 {
		t.Error("expected verbose phrases counted as redundant")
	}
}

func TestMeanScore(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "Short answer."),
		mkFinding("m2", "Another short answer."),
	}
	r := Analyze(fs)
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestLabCount(t *testing.T) {
	fs := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test2", Lab: "lab2"},
	}
	r := Analyze(fs)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestUnknownModel(t *testing.T) {
	fs := []model.Finding{{Summary: "test", Lab: "lab1"}}
	r := Analyze(fs)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown model, got %s", r.Models[0].Model)
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback",
		Evidence: []model.Evidence{
			{Marker: "marker1"},
			{Marker: "marker2"},
		},
	}
	et := evidenceText(f)
	if !strings.Contains(et, "marker1") {
		t.Error("expected marker in evidence text")
	}
}

func TestEvidenceFallback(t *testing.T) {
	f := model.Finding{Summary: "my summary"}
	et := evidenceText(f)
	if et != "my summary" {
		t.Errorf("expected summary fallback, got %q", et)
	}
}

func TestCountPhrases(t *testing.T) {
	text := "basically this is basically a test"
	c := countPhrases(text, []string{"basically"})
	if c != 2 {
		t.Errorf("expected 2, got %d", c)
	}
}

func TestCountPhrasesCase(t *testing.T) {
	text := "BASICALLY THIS IS Basically"
	c := countPhrases(text, []string{"basically"})
	if c != 2 {
		t.Errorf("expected 2 case-insensitive, got %d", c)
	}
}
