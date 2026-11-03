package citation

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: m},
		Summary: summary,
		Lab:     "lab1",
	}
}

func TestScoreResponseCleanText(t *testing.T) {
	score, detail := scoreResponse("This is a clean response with no citations.")
	if score != 100 {
		t.Errorf("expected 100, got %.1f", score)
	}
	if detail.TotalCitations != 0 {
		t.Errorf("expected 0 citations, got %d", detail.TotalCitations)
	}
}

func TestScoreResponseFakeURL(t *testing.T) {
	text := "See https://example.com/fake-paper for more info."
	score, detail := scoreResponse(text)
	if score >= 100 {
		t.Errorf("expected penalty, got %.1f", score)
	}
	if detail.FakeURLs != 1 {
		t.Errorf("expected 1 fake URL, got %d", detail.FakeURLs)
	}
}

func TestScoreResponseMultipleFakeURLs(t *testing.T) {
	text := "See https://example.com/a and https://placeholder.com/b for details."
	_, detail := scoreResponse(text)
	if detail.FakeURLs != 2 {
		t.Errorf("expected 2 fake URLs, got %d", detail.FakeURLs)
	}
}

func TestScoreResponseValidURL(t *testing.T) {
	text := "Reference: https://arxiv.org/abs/paper-id"
	score, detail := scoreResponse(text)
	if detail.FakeURLs != 0 {
		t.Errorf("expected 0 fake URLs, got %d", detail.FakeURLs)
	}
	if score != 100 {
		t.Errorf("expected 100 for valid URL only, got %.1f", score)
	}
}

func TestDetectBadDOIPlaceholder(t *testing.T) {
	text := "DOI: 10.0000/fake-article"
	issues := detectBadDOIs(text)
	if len(issues) != 1 {
		t.Fatalf("expected 1 bad DOI, got %d", len(issues))
	}
	if issues[0].Kind != "bad_doi" {
		t.Errorf("expected kind bad_doi, got %s", issues[0].Kind)
	}
}

func TestDetectBadDOIShortSuffix(t *testing.T) {
	text := "DOI: 10.1234/ab"
	issues := detectBadDOIs(text)
	if len(issues) != 1 {
		t.Fatalf("expected 1 bad DOI, got %d", len(issues))
	}
}

func TestDetectValidDOI(t *testing.T) {
	text := "DOI: 10.1038/nature12373"
	issues := detectBadDOIs(text)
	if len(issues) != 0 {
		t.Errorf("expected 0 bad DOIs, got %d", len(issues))
	}
}

func TestDetectBadYearFuture(t *testing.T) {
	text := "Published in 2099 by the Research Institute."
	issues := detectBadYears(text)
	if len(issues) != 1 {
		t.Fatalf("expected 1 bad year, got %d", len(issues))
	}
	if !strings.Contains(issues[0].Detail, "2099") {
		t.Errorf("expected detail to mention 2099, got %s", issues[0].Detail)
	}
}

func TestDetectBadYearAncient(t *testing.T) {
	text := "Originally described in 1200 by an alchemist."
	issues := detectBadYears(text)
	if len(issues) != 1 {
		t.Fatalf("expected 1 bad year, got %d", len(issues))
	}
}

func TestDetectValidYear(t *testing.T) {
	text := "Smith (2023) demonstrated this result."
	issues := detectBadYears(text)
	if len(issues) != 0 {
		t.Errorf("expected 0 bad years, got %d", len(issues))
	}
}

func TestDetectFabricatedAuthor(t *testing.T) {
	text := "According to Doe, J. this technique works well."
	issues := detectFabAuthors(text)
	if len(issues) != 1 {
		t.Fatalf("expected 1 fabricated author, got %d", len(issues))
	}
}

func TestDetectUnknownAuthor(t *testing.T) {
	text := "Unknown Author (2023) published this."
	issues := detectFabAuthors(text)
	if len(issues) != 1 {
		t.Fatalf("expected 1 fabricated author, got %d", len(issues))
	}
}

func TestDetectRealAuthor(t *testing.T) {
	text := "According to Vaswani et al. (2017), attention is all you need."
	issues := detectFabAuthors(text)
	if len(issues) != 0 {
		t.Errorf("expected 0 fabricated authors, got %d", len(issues))
	}
}

func TestScoreToGradeA(t *testing.T) {
	if g := scoreToGrade(95); g != "A" {
		t.Errorf("expected A, got %s", g)
	}
}

func TestScoreToGradeF(t *testing.T) {
	if g := scoreToGrade(30); g != "F" {
		t.Errorf("expected F, got %s", g)
	}
}

func TestScoreBounded(t *testing.T) {
	text := strings.Repeat("See https://example.com/fake ", 20)
	score, _ := scoreResponse(text)
	if score < 0 {
		t.Errorf("score should not be negative, got %.1f", score)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected empty report")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	fs := []model.Finding{
		finding("gpt-4", "Clean response with no citations."),
		finding("gpt-4", "Another clean response."),
	}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	fs := []model.Finding{
		finding("gpt-4", "Clean text."),
		finding("claude", "Reference https://example.com/fake for details."),
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostAccurate == "" {
		t.Error("expected most_accurate to be set")
	}
}

func TestAnalyzeModelWithPoorCitations(t *testing.T) {
	fs := []model.Finding{
		finding("bad-model", strings.Repeat("https://example.com/fake ", 10)),
	}
	r := Analyze(fs)
	if r.Models[0].PoorCitations != 1 {
		t.Errorf("expected 1 poor citation sample, got %d", r.Models[0].PoorCitations)
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	fs := []model.Finding{
		{Summary: "Clean text.", Lab: "lab1"},
	}
	r := Analyze(fs)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No citation data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	fs := []model.Finding{
		finding("gpt-4", "Reference https://example.com/fake-paper"),
	}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Citation Accuracy") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name")
	}
}

func TestFormatJSONValid(t *testing.T) {
	fs := []model.Finding{
		finding("gpt-4", "Clean."),
	}
	r := Analyze(fs)
	out := FormatJSON(r)
	var data map[string]any
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
	var data Report
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if data.MeanScore != 0 {
		t.Errorf("expected 0 mean score, got %.1f", data.MeanScore)
	}
}

func TestTruncate(t *testing.T) {
	if r := truncate("short", 10); r != "short" {
		t.Errorf("expected short, got %s", r)
	}
	if r := truncate("a very long string indeed", 10); len(r) > 14 {
		t.Errorf("expected truncated, got %s", r)
	}
}

func TestContainsAny(t *testing.T) {
	matched := containsAny("Hello World example test", []string{"world", "missing"})
	if len(matched) != 1 || matched[0] != "world" {
		t.Errorf("expected [world], got %v", matched)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	fs := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "Clean.", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "Clean.", Lab: "lab2"},
	}
	r := Analyze(fs)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeSortedByScore(t *testing.T) {
	fs := []model.Finding{
		finding("bad", strings.Repeat("https://example.com/x ", 5)),
		finding("good", "Clean response."),
	}
	r := Analyze(fs)
	if r.Models[0].AccuracyScore < r.Models[1].AccuracyScore {
		t.Error("expected models sorted by score descending")
	}
}
