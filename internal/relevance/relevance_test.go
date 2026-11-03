package relevance

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(mdl, lab, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mdl},
		Lab:     lab,
		Summary: summary,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatal("expected no models")
	}
}

func TestAnalyzeSingleRelevant(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "security vulnerability scan", "The security vulnerability scan found SQL injection.")}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].RelevanceScore < 70 {
		t.Errorf("relevant response should score well, got %.1f", r.Models[0].RelevanceScore)
	}
}

func TestTangentLowersScore(t *testing.T) {
	relevant := []model.Finding{mkFinding("m1", "test lab", "The test results show clear pass.")}
	tangent := []model.Finding{mkFinding("m1", "test lab", "By the way on a side note speaking of which incidentally as an aside this reminds me of something off topic.")}
	rr := Analyze(relevant)
	rt := Analyze(tangent)
	if rt.Models[0].RelevanceScore >= rr.Models[0].RelevanceScore {
		t.Error("tangent-heavy response should score lower")
	}
}

func TestEvasivePhrasesPenalize(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "lab", "That is a good question. It depends on many factors. It is hard to say. Opinions vary. There is no simple answer.")}
	r := Analyze(fs)
	if r.Models[0].Detail.TangentRate >= 0.9 {
		t.Error("evasive phrases should lower tangent rate score")
	}
}

func TestVaguePhrasesPenalize(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "lab", "Things and stuff sort of kind of more or less various aspects in some ways.")}
	r := Analyze(fs)
	if r.Models[0].Detail.SpecificityScr >= 0.8 {
		t.Errorf("vague phrases should lower specificity, got %.2f", r.Models[0].Detail.SpecificityScr)
	}
}

func TestTermOverlapEmpty(t *testing.T) {
	shared, cov := termOverlap("", "some response text")
	if shared != 0 || cov != 1.0 {
		t.Errorf("empty prompt should yield 0 shared, 1.0 coverage; got %d, %.2f", shared, cov)
	}
}

func TestTermOverlapMatch(t *testing.T) {
	shared, _ := termOverlap("security vulnerability detection", "The security vulnerability was detected successfully.")
	if shared < 2 {
		t.Errorf("expected at least 2 shared terms, got %d", shared)
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "code review", "The code review found buffer overflow."),
		mkFinding("m2", "code review", "By the way on a side note things are complicated. It depends."),
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
	if scoreToGrade(95) != "A" {
		t.Error("expected A")
	}
}

func TestGradeB(t *testing.T) {
	if scoreToGrade(85) != "B" {
		t.Error("expected B")
	}
}

func TestGradeC(t *testing.T) {
	if scoreToGrade(75) != "C" {
		t.Error("expected C")
	}
}

func TestGradeD(t *testing.T) {
	if scoreToGrade(65) != "D" {
		t.Error("expected D")
	}
}

func TestGradeF(t *testing.T) {
	if scoreToGrade(45) != "F" {
		t.Error("expected F")
	}
}

func TestScoreBounded(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "lab", strings.Repeat("by the way on a side note things stuff kind of sort of ", 30))}
	r := Analyze(fs)
	if r.Models[0].RelevanceScore < 0 || r.Models[0].RelevanceScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].RelevanceScore)
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "lab", "Relevant response.")}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Relevance") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "m1") {
		t.Error("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No relevance data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "lab", "Test response.")}
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
	fs := []model.Finding{mkFinding("m1", "lab", "By the way on a side note speaking of which incidentally as an aside changing the subject.")}
	r := Analyze(fs)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns for heavy tangent use")
	}
}

func TestSharedTermsBoostSpecificity(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "network security firewall configuration audit", "The network security firewall configuration audit revealed open ports and misconfigured rules.")}
	r := Analyze(fs)
	if r.Models[0].Detail.SharedTerms < 3 {
		t.Errorf("expected shared terms >= 3, got %d", r.Models[0].Detail.SharedTerms)
	}
}

func TestMeanScore(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "lab1", "Relevant response."),
		mkFinding("m2", "lab2", "Another relevant answer."),
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

func TestContentWordsFilters(t *testing.T) {
	words := contentWords("The quick brown fox is a great animal")
	for _, w := range words {
		if stopWords[w] {
			t.Errorf("stop word %q not filtered", w)
		}
	}
}

func TestCountPhrasesCase(t *testing.T) {
	c := countPhrases("BY THE WAY this is By The Way", []string{"by the way"})
	if c != 2 {
		t.Errorf("expected 2 case-insensitive, got %d", c)
	}
}
