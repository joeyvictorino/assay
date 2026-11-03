package analogy

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

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestScoreHigh(t *testing.T) {
	text := "think of it like a mailbox. similar to how letters arrive. just like in real life. " +
		"this analogy helps because the parallel here is clear. a familiar example everyone knows. " +
		"a fresh perspective on the problem."
	score, detail := scoreResponse(text, "")
	if score < 60 {
		t.Fatalf("expected score >= 60, got %.1f", score)
	}
	if detail.AnalogyCount < 2 {
		t.Fatalf("expected analogy_count >= 2, got %d", detail.AnalogyCount)
	}
}

func TestScoreLow(t *testing.T) {
	score, _ := scoreResponse("hello world", "")
	if score > 60 {
		t.Fatalf("expected score <= 60, got %.1f", score)
	}
}

func TestBadAnalogyPenalty(t *testing.T) {
	good := "think of it like a river. similar to water flowing downstream."
	bad := good + " the analogy breaks down here. false equivalence. this is a stretch."
	scoreGood, _ := scoreResponse(good, "")
	scoreBad, _ := scoreResponse(bad, "")
	if scoreBad >= scoreGood {
		t.Fatalf("bad analogy should lower score: good=%.1f bad=%.1f", scoreGood, scoreBad)
	}
}

func TestGradeMapping(t *testing.T) {
	cases := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {45, "F"},
	}
	for _, c := range cases {
		if g := scoreToGrade(c.score); g != c.grade {
			t.Fatalf("score %.0f: expected %s, got %s", c.score, c.grade, g)
		}
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "think of it like a tree with branches. similar to how data flows."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Fatalf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.BestModel != "gpt-4" {
		t.Fatalf("expected best=gpt-4")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "think of it like a mailbox. similar to how letters arrive."),
		finding("claude", "hello world"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel != "gpt-4" {
		t.Fatalf("expected gpt-4 as best, got %s", r.BestModel)
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	f := model.Finding{Summary: "think of it like a factory. similar to an assembly line."}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Fatalf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No analogy data") {
		t.Fatal("expected empty state message")
	}
}

func TestFormatTextWithData(t *testing.T) {
	r := Report{
		Models: []ModelAnalogy{{
			Model: "test", AnalogyScore: 75.0, Grade: "C",
			Detail: AnalogyDetail{AnalogyCount: 3, Relevance: 0.5},
		}},
		BestModel: "test", MeanScore: 75.0,
	}
	text := FormatText(r)
	if !strings.Contains(text, "test") {
		t.Fatal("expected model name in output")
	}
	if !strings.Contains(text, "75.0") {
		t.Fatal("expected score in output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Report{Models: []ModelAnalogy{{Model: "x", AnalogyScore: 80.0}}}
	data := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Models[0].Model != "x" {
		t.Fatal("roundtrip failed")
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback",
		Evidence: []model.Evidence{
			{Marker: "think of it like a pipeline"},
		},
	}
	text := evidenceText(f)
	if text != "think of it like a pipeline" {
		t.Fatalf("expected marker text, got %q", text)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "no markers here"}
	text := evidenceText(f)
	if text != "no markers here" {
		t.Fatalf("expected summary fallback, got %q", text)
	}
}

func TestCountPhrases(t *testing.T) {
	text := "think of it like a tree. similar to a forest. think of it like a river."
	count := countPhrases(text, analogyPhrases)
	if count < 3 {
		t.Fatalf("expected >= 3 matches, got %d", count)
	}
}

func TestLabCounting(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m"}, Summary: "a", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m"}, Summary: "b", Lab: "lab2"},
		{Model: model.ModelRef{Model: "m"}, Summary: "c", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}
