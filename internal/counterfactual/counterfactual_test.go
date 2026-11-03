package counterfactual

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestCountPhrases(t *testing.T) {
	if n := countPhrases("What if we changed? What if again?", alternativePhrases); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
	if n := countPhrases("nothing relevant here", alternativePhrases); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
	if n := countPhrases("", alternativePhrases); n != 0 {
		t.Errorf("expected 0 for empty, got %d", n)
	}
}

func TestScoreResponseHigh(t *testing.T) {
	text := "What if we chose differently? Alternatively, suppose instead the premise were varied. " +
		"The consequence would be a ripple effect, leading to a shifted outcome compared to the original. " +
		"Because of this change, the causal chain alters. The causal link directly causes divergence."
	s, d := scoreResponse(text)
	if s < 50 {
		t.Errorf("expected high score, got %.1f", s)
	}
	if d.AlternativeGeneration == 0 {
		t.Error("expected nonzero alternative generation")
	}
	if d.CausalReasoning == 0 {
		t.Error("expected nonzero causal reasoning")
	}
}

func TestScoreResponseLow(t *testing.T) {
	s, _ := scoreResponse("the sky is blue and water is wet")
	if s > 40 {
		t.Errorf("expected low score, got %.1f", s)
	}
}

func TestScoreResponseSpeculativePenalty(t *testing.T) {
	base := "What if we changed? Alternatively, suppose instead."
	penalized := base + " Maybe something. It could be anything. Who knows what. Purely random."
	s1, _ := scoreResponse(base)
	s2, _ := scoreResponse(penalized)
	if s2 >= s1 {
		t.Errorf("speculative penalty not applied: base=%.1f penalized=%.1f", s1, s2)
	}
}

func TestScoreResponseContradictoryPenalty(t *testing.T) {
	base := "What if we changed the approach?"
	penalized := base + " Both would and would not happen. Contradicts itself."
	s1, _ := scoreResponse(base)
	s2, _ := scoreResponse(penalized)
	if s2 >= s1 {
		t.Errorf("contradictory penalty not applied: base=%.1f penalized=%.1f", s1, s2)
	}
}

func TestScoreResponseBounds(t *testing.T) {
	s1, _ := scoreResponse("")
	if s1 < 0 || s1 > 100 {
		t.Errorf("score out of bounds: %.1f", s1)
	}
	heavy := strings.Repeat("maybe something who knows what purely random impossible to say ", 20)
	s2, _ := scoreResponse(heavy)
	if s2 < 0 {
		t.Errorf("score below 0: %.1f", s2)
	}
}

func TestScoreToGrade(t *testing.T) {
	cases := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {40, "F"},
	}
	for _, c := range cases {
		if g := scoreToGrade(c.score); g != c.grade {
			t.Errorf("score %.0f: expected %s, got %s", c.score, c.grade, g)
		}
	}
}

func TestEvidenceTextShort(t *testing.T) {
	if e := evidenceText("short"); e != "short" {
		t.Errorf("unexpected: %s", e)
	}
}

func TestEvidenceTextLong(t *testing.T) {
	long := strings.Repeat("a", 200)
	e := evidenceText(long)
	if !strings.HasSuffix(e, "...") {
		t.Error("expected truncation")
	}
	if len(e) > 124 {
		t.Errorf("too long: %d", len(e))
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected empty models, got %d", len(r.Models))
	}
	if r.Title != "Counterfactual Reasoning Analysis" {
		t.Errorf("unexpected title: %s", r.Title)
	}
}

func TestAnalyzeSingleFinding(t *testing.T) {
	fs := []model.Finding{
		{Summary: "What if we changed? The consequence would be major.", Model: model.ModelRef{Model: "gpt-4"}, Lab: "lab1"},
	}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Count != 1 {
		t.Errorf("expected count 1, got %d", r.Models[0].Count)
	}
	if r.Models[0].Score <= 0 {
		t.Error("expected positive score")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	fs := []model.Finding{
		{Summary: "What if scenario A?", Model: model.ModelRef{Model: "m1"}, Lab: "l1"},
		{Summary: "Alternatively, scenario B.", Model: model.ModelRef{Model: "m2"}, Lab: "l1"},
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(r.Models))
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	fs := []model.Finding{
		{Summary: "basic text", Model: model.ModelRef{Model: ""}, Lab: ""},
	}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1, got %d", len(r.Models))
	}
}

func TestAnalyzeLabGrouping(t *testing.T) {
	fs := []model.Finding{
		{Summary: "what if A", Model: model.ModelRef{Model: "m1"}, Lab: "lab1"},
		{Summary: "what if B", Model: model.ModelRef{Model: "m1"}, Lab: "lab2"},
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Errorf("expected 2 buckets, got %d", len(r.Models))
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	fs := []model.Finding{
		{
			Summary: "maybe something it could be anything who knows what purely random maybe something no way to tell impossible to say",
			Model:   model.ModelRef{Model: "m1"},
			Lab:     "l1",
		},
	}
	r := Analyze(fs)
	found := false
	for _, c := range r.Models[0].Concerns {
		if strings.Contains(c, "speculative") {
			found = true
		}
	}
	if !found {
		t.Error("expected speculative concern")
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	fs := []model.Finding{
		{Summary: "what if we changed? alternatively suppose instead", Model: model.ModelRef{Model: "m1"}, Lab: "l1"},
		{Summary: "plain text nothing special here", Model: model.ModelRef{Model: "m1"}, Lab: "l1"},
	}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Score <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestFormatText(t *testing.T) {
	r := Report{
		Title: "Counterfactual Reasoning Analysis",
		Models: []ModelCounterfactual{
			{Model: "m1", Lab: "l1", Score: 75.5, Grade: "C", Count: 3},
		},
	}
	text := FormatText(r)
	if !strings.Contains(text, "Counterfactual") {
		t.Error("expected title in output")
	}
	if !strings.Contains(text, "75.5") {
		t.Error("expected score in output")
	}
}

func TestFormatTextWithConcerns(t *testing.T) {
	r := Report{
		Title: "Counterfactual Reasoning Analysis",
		Models: []ModelCounterfactual{
			{Model: "m1", Lab: "l1", Score: 50, Grade: "F", Count: 1,
				Concerns: []string{"high speculative count (5)"}},
		},
	}
	text := FormatText(r)
	if !strings.Contains(text, "speculative") {
		t.Error("expected concern in output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Report{
		Title:  "Counterfactual Reasoning Analysis",
		Models: []ModelCounterfactual{{Model: "m1", Lab: "l1", Score: 80, Grade: "B", Count: 2}},
	}
	j := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(j), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Title != r.Title {
		t.Errorf("title mismatch: %s", parsed.Title)
	}
}

func TestPremiseVariation(t *testing.T) {
	text := "Changing the assumption leads to different results. Altering the condition modifying the starting point."
	_, d := scoreResponse(text)
	if d.PremiseVariation == 0 {
		t.Error("expected nonzero premise variation")
	}
}

func TestOutcomeComparison(t *testing.T) {
	text := "Compared to the original the delta between outcomes differs from the baseline."
	_, d := scoreResponse(text)
	if d.OutcomeComparison == 0 {
		t.Error("expected nonzero outcome comparison")
	}
}

func TestScoreDimensionCapped(t *testing.T) {
	heavy := strings.Repeat("what if ", 50)
	_, d := scoreResponse(heavy)
	if d.AlternativeGeneration > 100 {
		t.Errorf("dimension exceeds 100: %.1f", d.AlternativeGeneration)
	}
}

func TestRoundTrip(t *testing.T) {
	s, _ := scoreResponse("what if alternatively suppose instead the consequence would be leading to because of this change")
	if math.IsNaN(s) || math.IsInf(s, 0) {
		t.Errorf("invalid score: %f", s)
	}
}
