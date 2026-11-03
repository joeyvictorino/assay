package factprecision

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(mdl, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mdl},
		Lab:     "lab1",
		Summary: summary,
	}
}

func TestCountPhrasesSpecific(t *testing.T) {
	text := "specifically for example including this particular case"
	c := countPhrases(text, specificClaimPhrases)
	if c < 3 {
		t.Errorf("expected >=3 specific phrases, got %d", c)
	}
}

func TestCountPhrasesVerifiable(t *testing.T) {
	text := "measured and verified through published benchmarks"
	c := countPhrases(text, verifiableFactPhrases)
	if c < 2 {
		t.Errorf("expected >=2 verifiable phrases, got %d", c)
	}
}

func TestCountPhrasesVague(t *testing.T) {
	text := "some people say that it is sort of kind of right"
	c := countPhrases(text, vagueClaimPhrases)
	if c < 2 {
		t.Errorf("expected >=2 vague phrases, got %d", c)
	}
}

func TestCountPhrasesOvergeneralize(t *testing.T) {
	text := "always works and is universally accepted beyond question"
	c := countPhrases(text, overgeneralizePhrases)
	if c < 2 {
		t.Errorf("expected >=2 overgeneralize phrases, got %d", c)
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, _ := scoreResponse("")
	if score != 0 {
		t.Errorf("expected 0 for empty, got %f", score)
	}
}

func TestScoreResponseHighPrecision(t *testing.T) {
	text := "Specifically, according to the NIST documentation published in RFC 7519, the measured performance increased by 45 percent based on verified benchmarks."
	score, detail := scoreResponse(text)
	if score <= 50 {
		t.Errorf("expected >50 for precise text, got %f", score)
	}
	if detail.SpecificClaimCount == 0 {
		t.Error("expected specific claims")
	}
}

func TestScoreResponseLowPrecision(t *testing.T) {
	text := "Some people say that it sort of kind of presumably always works and is universally accepted beyond question."
	score, detail := scoreResponse(text)
	if score >= 50 {
		t.Errorf("expected <50 for vague text, got %f", score)
	}
	if detail.VagueClaimCount == 0 {
		t.Error("expected vague claims")
	}
}

func TestScoreResponseBounded(t *testing.T) {
	text := strings.Repeat("specifically measured verified published in ", 100)
	score, _ := scoreResponse(text)
	if score > 100 || score < 0 {
		t.Errorf("score out of bounds: %f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	cases := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {45, "F"},
	}
	for _, c := range cases {
		if g := scoreToGrade(c.score); g != c.grade {
			t.Errorf("score %.0f: expected %s, got %s", c.score, c.grade, g)
		}
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected empty report")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "Specifically, according to NIST guidelines, the measured performance verified."),
		finding("gpt-4", "For example, including the documented evidence shows the results."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
	if r.MostPrecise != "gpt-4" {
		t.Errorf("expected most precise gpt-4, got %s", r.MostPrecise)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	findings := []model.Finding{
		finding("model-a", "Specifically, the measured evidence shows verified results per the documentation."),
		finding("model-b", "Some people say it sort of kind of presumably works."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostPrecise == r.LeastPrecise {
		t.Error("most and least should differ")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "Specifically documented."},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "test"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab2", Summary: "test"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeSorted(t *testing.T) {
	findings := []model.Finding{
		finding("low", "some people say sort of kind of presumably."),
		finding("high", "Specifically, according to verified measured benchmarks show published documented."),
	}
	r := Analyze(findings)
	if r.Models[0].PrecisionScore < r.Models[1].PrecisionScore {
		t.Error("expected sorted by score descending")
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "Specifically documented."),
		finding("m2", "Specifically documented."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestAnalyzeHighLowCounts(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "Specifically according to NIST measured verified published in RFC documented evidence shows benchmarks show."),
		finding("m1", "some people say sort of kind of presumably always works universally accepted beyond question."),
	}
	r := Analyze(findings)
	if r.Models[0].HighPrecision == 0 && r.Models[0].LowPrecision == 0 {
		t.Error("expected at least one high or low precision count")
	}
}

func TestEvidenceTextMarker(t *testing.T) {
	f := model.Finding{
		Summary:  "summary text",
		Evidence: []model.Evidence{{Marker: "marker text"}},
	}
	if evidenceText(f) != "marker text" {
		t.Error("expected marker text")
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "summary text"}
	if evidenceText(f) != "summary text" {
		t.Error("expected summary text fallback")
	}
}

func TestFormatTextOutput(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "Specifically documented evidence shows."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Factual Precision") {
		t.Error("expected 'Factual Precision' header")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No findings") {
		t.Error("expected 'No findings' message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "Specifically documented."),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Error("expected 1 model in JSON")
	}
}

func TestFormatJSONEmpty(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}
