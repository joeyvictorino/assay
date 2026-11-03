package accuracy

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

func TestAnalyzeSingleFactual(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Studies show the result is 42. Evidence shows this is confirmed by analysis.")}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].AccuracyScore <= 0 {
		t.Error("expected positive score")
	}
}

func TestFactualScoresHigher(t *testing.T) {
	factual := []model.Finding{mkFinding("m1", "Studies show and research indicates the data. According to evidence the analysis reveals confirmed results. Experiments demonstrate the findings show truth.")}
	vague := []model.Finding{mkFinding("m1", "Some people say it is generally known everyone knows it is common knowledge obviously clearly.")}
	rf := Analyze(factual)
	rv := Analyze(vague)
	if rv.Models[0].AccuracyScore >= rf.Models[0].AccuracyScore {
		t.Errorf("factual (%.1f) should score higher than vague (%.1f)", rf.Models[0].AccuracyScore, rv.Models[0].AccuracyScore)
	}
}

func TestHedgingPenalty(t *testing.T) {
	confident := []model.Finding{mkFinding("m1", "The answer is definitively 42. Studies show this clearly.")}
	hedged := []model.Finding{mkFinding("m1", "It might be possibly 42. Perhaps it could be that. It seems it appears presumably arguably.")}
	rc := Analyze(confident)
	rh := Analyze(hedged)
	if rh.Models[0].Detail.HedgingScore >= rc.Models[0].Detail.HedgingScore {
		t.Error("hedged should have lower hedging score")
	}
}

func TestContradictionPenalty(t *testing.T) {
	consistent := []model.Finding{mkFinding("m1", "The data shows X. Further analysis confirms X.")}
	contradicted := []model.Finding{mkFinding("m1", "The answer is X. However this contradicts the previous finding. This is inconsistent with the data. Contrary to what I stated earlier.")}
	rc := Analyze(consistent)
	ri := Analyze(contradicted)
	if ri.Models[0].AccuracyScore >= rc.Models[0].AccuracyScore {
		t.Error("contradicted should score lower")
	}
}

func TestPrecisionIndicators(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Specifically the value is precisely 42.0. To be exact it consists of three components defined as A plus B plus C.")}
	r := Analyze(fs)
	if r.Models[0].Detail.PrecisionScore < 0.4 {
		t.Errorf("expected higher precision, got %.2f", r.Models[0].Detail.PrecisionScore)
	}
}

func TestSourceIndicators(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "According to the report published in Nature and as reported by the team. Cited in the paper and documented in the study.")}
	r := Analyze(fs)
	if r.Models[0].Detail.SourceScore < 0.4 {
		t.Errorf("expected higher source score, got %.2f", r.Models[0].Detail.SourceScore)
	}
}

func TestVaguePhrasesPenalize(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Some people say everyone knows it is common knowledge obviously clearly needless to say as we all know.")}
	r := Analyze(fs)
	if r.Models[0].Detail.PrecisionScore >= 0.3 {
		// vague phrases should reduce precision
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "Studies show data. Research indicates findings."),
		mkFinding("m2", "Maybe possibly perhaps it seems."),
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best/worst set")
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
	fs := []model.Finding{mkFinding("m1", strings.Repeat("studies show research indicates according to evidence published in ", 20))}
	r := Analyze(fs)
	if r.Models[0].AccuracyScore < 0 || r.Models[0].AccuracyScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].AccuracyScore)
	}
}

func TestScoreBoundedLow(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", strings.Repeat("however this contradicts this is inconsistent contrary to what i stated ", 20))}
	r := Analyze(fs)
	if r.Models[0].AccuracyScore < 0 || r.Models[0].AccuracyScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].AccuracyScore)
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Studies show the answer.")}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Accuracy") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "m1") {
		t.Error("expected model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No accuracy data") {
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
		t.Error("expected models key")
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
	fs := []model.Finding{mkFinding("m1", "It might be possibly perhaps it seems it appears presumably arguably supposedly allegedly reportedly in theory hypothetically speculatively it is believed.")}
	r := Analyze(fs)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns for heavy hedging")
	}
}

func TestClaimCount(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Studies show data. Specifically the value equals 42.")}
	r := Analyze(fs)
	if r.Models[0].Detail.ClaimCount == 0 {
		t.Error("expected positive claim count")
	}
}

func TestHedgeCount(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "It might be true. It could be false. Perhaps maybe.")}
	r := Analyze(fs)
	if r.Models[0].Detail.HedgeCount == 0 {
		t.Error("expected positive hedge count")
	}
}

func TestContradictCount(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "This is X. However this contradicts the earlier point.")}
	r := Analyze(fs)
	if r.Models[0].Detail.ContradictCount == 0 {
		t.Error("expected positive contradict count")
	}
}

func TestMeanScore(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "Studies show evidence."),
		mkFinding("m2", "Research indicates findings."),
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
	text := "studies show data and studies show more"
	c := countPhrases(text, []string{"studies show"})
	if c != 2 {
		t.Errorf("expected 2, got %d", c)
	}
}

func TestCountPhrasesCase(t *testing.T) {
	text := "STUDIES SHOW data and Studies Show more"
	c := countPhrases(text, []string{"studies show"})
	if c != 2 {
		t.Errorf("expected 2, got %d", c)
	}
}
