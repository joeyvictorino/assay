package contradict

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

func TestScoreConsistent(t *testing.T) {
	text := "as i mentioned earlier, this confirms the previous finding. consistent with the data."
	score, detail := scoreResponse(text, "")
	if score < 70 {
		t.Fatalf("expected score >= 70 for consistent text, got %.1f", score)
	}
	if detail.ConsistencyScore < 0.5 {
		t.Fatalf("expected consistency >= 0.5, got %.2f", detail.ConsistencyScore)
	}
}

func TestScoreContradictory(t *testing.T) {
	text := "the answer is yes. however, this contradicts the evidence. actually, the opposite is true. " +
		"this is not true. i was mistaken in my earlier analysis."
	score, detail := scoreResponse(text, "")
	if detail.ContradictionCount < 1 {
		t.Fatalf("expected contradictions >= 1, got %d", detail.ContradictionCount)
	}
	if score > 60 {
		t.Fatalf("expected score < 60 for contradictory text, got %.1f", score)
	}
}

func TestContradictionLowersScore(t *testing.T) {
	clean := "the data shows a clear trend. as i mentioned, this supports the hypothesis."
	dirty := clean + " however, this contradicts the data. actually, the opposite is true."
	scoreClean, _ := scoreResponse(clean, "")
	scoreDirty, _ := scoreResponse(dirty, "")
	if scoreDirty >= scoreClean {
		t.Fatalf("contradictions should lower score: clean=%.1f dirty=%.1f", scoreClean, scoreDirty)
	}
}

func TestHedgePenalty(t *testing.T) {
	clear := "the answer is definitively yes based on the evidence."
	hedged := "it depends on context. it is hard to say. it could go either way. possibly maybe."
	scoreClear, _ := scoreResponse(clear, "")
	scoreHedged, _ := scoreResponse(hedged, "")
	if scoreHedged >= scoreClear {
		t.Fatalf("hedging should lower score: clear=%.1f hedged=%.1f", scoreClear, scoreHedged)
	}
}

func TestNegationDetection(t *testing.T) {
	text := "this is not true. that is incorrect. this is wrong."
	_, detail := scoreResponse(text, "")
	if detail.NegationCount < 2 {
		t.Fatalf("expected negation_count >= 2, got %d", detail.NegationCount)
	}
}

func TestRetractionDetection(t *testing.T) {
	text := "i apologize for the confusion. i need to correct my earlier answer. upon reflection, a better answer would be different."
	_, detail := scoreResponse(text, "")
	if detail.RetractionCount < 2 {
		t.Fatalf("expected retraction_count >= 2, got %d", detail.RetractionCount)
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
		finding("gpt-4", "as i mentioned, this confirms the analysis. consistent with expectations."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Fatalf("expected gpt-4, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "as i mentioned, consistent with the evidence. this confirms it."),
		finding("claude", "however, this contradicts. actually, the opposite is true. this is not true."),
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
	f := model.Finding{Summary: "simple consistent response"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Fatalf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No contradiction data") {
		t.Fatal("expected empty state message")
	}
}

func TestFormatTextWithData(t *testing.T) {
	r := Report{
		Models: []ModelContradict{{
			Model: "test", ContradictScore: 80.0, Grade: "B",
			Detail: ContradictDetail{ContradictionCount: 1, ConsistencyScore: 0.6},
		}},
		BestModel: "test", MeanScore: 80.0,
	}
	text := FormatText(r)
	if !strings.Contains(text, "test") {
		t.Fatal("expected model name in output")
	}
	if !strings.Contains(text, "80.0") {
		t.Fatal("expected score in output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Report{Models: []ModelContradict{{Model: "x", ContradictScore: 75.0}}}
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
			{Marker: "consistent with expectations"},
		},
	}
	text := evidenceText(f)
	if text != "consistent with expectations" {
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

func TestScoreBounds(t *testing.T) {
	score, _ := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Fatalf("score out of bounds: %.1f", score)
	}
}
