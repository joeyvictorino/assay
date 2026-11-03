package deduction

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestCountPhrases(t *testing.T) {
	if n := countPhrases("Given that X, therefore Y. Therefore Z.", chainingPhrases); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
	if n := countPhrases("nothing here", chainingPhrases); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
	if n := countPhrases("", chainingPhrases); n != 0 {
		t.Errorf("expected 0 for empty, got %d", n)
	}
}

func TestScoreResponseHigh(t *testing.T) {
	text := "Given that all mammals breathe, therefore it follows that dogs breathe. " +
		"The contrapositive shows if not breathing then not a mammal. " +
		"Since every dog is a mammal, the deduction holds. Valid conclusion reached by modus tollens."
	s, d := scoreResponse(text)
	if s < 40 {
		t.Errorf("expected high score, got %.1f", s)
	}
	if d.PremiseIdentification == 0 {
		t.Error("expected nonzero premise identification")
	}
	if d.LogicalChaining == 0 {
		t.Error("expected nonzero logical chaining")
	}
}

func TestScoreResponseLow(t *testing.T) {
	s, _ := scoreResponse("the weather is nice today and cats are cute")
	if s > 40 {
		t.Errorf("expected low score, got %.1f", s)
	}
}

func TestScoreResponseFallacyPenalty(t *testing.T) {
	base := "Given that X, therefore Y. It follows that Z."
	penalized := base + " Affirming the consequent. Non sequitur. Begging the question. Circular reasoning."
	s1, _ := scoreResponse(base)
	s2, _ := scoreResponse(penalized)
	if s2 >= s1 {
		t.Errorf("fallacy penalty not applied: base=%.1f penalized=%.1f", s1, s2)
	}
}

func TestScoreResponseVaguePenalty(t *testing.T) {
	base := "Therefore it follows that the conclusion holds."
	penalized := base + " Probably maybe. It might or might not. Hard to say."
	s1, _ := scoreResponse(base)
	s2, _ := scoreResponse(penalized)
	if s2 >= s1 {
		t.Errorf("vague penalty not applied: base=%.1f penalized=%.1f", s1, s2)
	}
}

func TestScoreResponseBounds(t *testing.T) {
	s1, _ := scoreResponse("")
	if s1 < 0 || s1 > 100 {
		t.Errorf("score out of bounds: %.1f", s1)
	}
	heavy := strings.Repeat("probably maybe hard to say unclear logic reasoning is fuzzy ", 20)
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
	long := strings.Repeat("b", 200)
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
	if r.Title != "Deductive Reasoning Analysis" {
		t.Errorf("unexpected title: %s", r.Title)
	}
}

func TestAnalyzeSingleFinding(t *testing.T) {
	fs := []model.Finding{
		{Summary: "Given that X, therefore Y follows.", Model: model.ModelRef{Model: "gpt-4"}, Lab: "lab1"},
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
		{Summary: "Given that A, therefore B.", Model: model.ModelRef{Model: "m1"}, Lab: "l1"},
		{Summary: "It follows that C.", Model: model.ModelRef{Model: "m2"}, Lab: "l1"},
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
		{Summary: "therefore A", Model: model.ModelRef{Model: "m1"}, Lab: "lab1"},
		{Summary: "therefore B", Model: model.ModelRef{Model: "m1"}, Lab: "lab2"},
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Errorf("expected 2 buckets, got %d", len(r.Models))
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	fs := []model.Finding{
		{
			Summary: "affirming the consequent non sequitur begging the question circular reasoning affirming the consequent denying the antecedent",
			Model:   model.ModelRef{Model: "m1"},
			Lab:     "l1",
		},
	}
	r := Analyze(fs)
	found := false
	for _, c := range r.Models[0].Concerns {
		if strings.Contains(c, "fallacy") {
			found = true
		}
	}
	if !found {
		t.Error("expected fallacy concern")
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	fs := []model.Finding{
		{Summary: "given that X therefore Y it follows that Z", Model: model.ModelRef{Model: "m1"}, Lab: "l1"},
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
		Title: "Deductive Reasoning Analysis",
		Models: []ModelDeduction{
			{Model: "m1", Lab: "l1", Score: 82.3, Grade: "B", Count: 5},
		},
	}
	text := FormatText(r)
	if !strings.Contains(text, "Deductive") {
		t.Error("expected title in output")
	}
	if !strings.Contains(text, "82.3") {
		t.Error("expected score in output")
	}
}

func TestFormatTextWithConcerns(t *testing.T) {
	r := Report{
		Title: "Deductive Reasoning Analysis",
		Models: []ModelDeduction{
			{Model: "m1", Lab: "l1", Score: 45, Grade: "F", Count: 1,
				Concerns: []string{"high fallacy count (5)"}},
		},
	}
	text := FormatText(r)
	if !strings.Contains(text, "fallacy") {
		t.Error("expected concern in output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Report{
		Title:  "Deductive Reasoning Analysis",
		Models: []ModelDeduction{{Model: "m1", Lab: "l1", Score: 88, Grade: "B", Count: 3}},
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

func TestContrapositive(t *testing.T) {
	text := "The contrapositive shows that modus tollens applies. By contradiction we see the inverse."
	_, d := scoreResponse(text)
	if d.ContrapositivUse == 0 {
		t.Error("expected nonzero contrapositive use")
	}
}

func TestSyllogistic(t *testing.T) {
	text := "Since every X is Y and all A are B, the major premise establishes the universal affirmative."
	_, d := scoreResponse(text)
	if d.SyllogisticReasoning == 0 {
		t.Error("expected nonzero syllogistic reasoning")
	}
}

func TestScoreDimensionCapped(t *testing.T) {
	heavy := strings.Repeat("therefore ", 50)
	_, d := scoreResponse(heavy)
	if d.LogicalChaining > 100 {
		t.Errorf("dimension exceeds 100: %.1f", d.LogicalChaining)
	}
}

func TestRoundTrip(t *testing.T) {
	s, _ := scoreResponse("given that X therefore Y which implies Z valid conclusion by modus tollens")
	if math.IsNaN(s) || math.IsInf(s, 0) {
		t.Errorf("invalid score: %f", s)
	}
}
