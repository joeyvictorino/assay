package accessibility

import (
	"math"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestCountPhrases(t *testing.T) {
	tests := []struct {
		text    string
		phrases []string
		want    int
	}{
		{"in simple terms put simply", plainPhrases, 2},
		{"nothing here", plainPhrases, 0},
		{"first second third finally", structurePhrases, 4},
		{"for example such as for instance", examplePhrases, 3},
		{"if you are new to for beginners", audiencePhrases, 2},
		{"eigenvalue heuristic orthogonal", jargonPhrases, 3},
		{"it is trivially obvious as is well known", complexPhrases, 2},
	}
	for _, tt := range tests {
		got := countPhrases(tt.text, tt.phrases)
		if got != tt.want {
			t.Errorf("countPhrases(%q) = %d, want %d", tt.text[:20], got, tt.want)
		}
	}
}

func TestCountPhrasesCase(t *testing.T) {
	got := countPhrases("IN SIMPLE TERMS Put Simply ESSENTIALLY", plainPhrases)
	if got != 3 {
		t.Errorf("case insensitive count = %d, want 3", got)
	}
}

func TestScoreResponseBaseline(t *testing.T) {
	score, detail := scoreResponse("hello world", "")
	if score < 30 || score > 70 {
		t.Errorf("baseline score = %.1f, want 30-70", score)
	}
	if detail.AccessibleCount != 0 {
		t.Errorf("accessible count = %d, want 0", detail.AccessibleCount)
	}
}

func TestScoreResponseAccessible(t *testing.T) {
	text := "In simple terms, put simply, this means the basic idea. " +
		"First, second, third, finally, to recap. " +
		"For example, such as, for instance, to illustrate. " +
		"If you are new to this, for beginners, no prior knowledge needed."
	score, detail := scoreResponse(text, "")
	if score < 55 {
		t.Errorf("accessible score = %.1f, want >= 55", score)
	}
	if detail.AccessibleCount < 10 {
		t.Errorf("accessible count = %d, want >= 10", detail.AccessibleCount)
	}
}

func TestScoreResponseJargon(t *testing.T) {
	text := "The eigenvalue of the heuristic is orthogonal to the idempotent polymorphism monadic functorial isomorphic approach."
	_, detail := scoreResponse(text, "")
	if detail.JargonCount < 5 {
		t.Errorf("jargon count = %d, want >= 5", detail.JargonCount)
	}
	if detail.JargonAvoidance > 0.5 {
		t.Errorf("jargon avoidance = %.2f, want <= 0.5", detail.JargonAvoidance)
	}
}

func TestScoreResponseComplex(t *testing.T) {
	text := "It is trivially obvious that as is well known the proof is left by induction on without loss of generality."
	_, detail := scoreResponse(text, "")
	if detail.ComplexCount < 4 {
		t.Errorf("complex count = %d, want >= 4", detail.ComplexCount)
	}
}

func TestScoreResponseClamped(t *testing.T) {
	score, _ := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Errorf("score = %.1f, want 0-100", score)
	}
}

func TestScoreResponseRounding(t *testing.T) {
	score, detail := scoreResponse("in simple terms for example", "")
	remainder := math.Mod(score*10, 1.0)
	if remainder > 0.01 {
		t.Errorf("score %.1f not rounded to 1 decimal", score)
	}
	r2 := math.Mod(detail.PlainLanguage*100, 1.0)
	if r2 > 0.01 {
		t.Errorf("plain language %.2f not rounded to 2 decimals", detail.PlainLanguage)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{95, "A"}, {90, "A"},
		{85, "B"}, {80, "B"},
		{75, "C"}, {70, "C"},
		{65, "D"}, {60, "D"},
		{55, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%.0f) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestEvidenceTextMarkers(t *testing.T) {
	f := model.Finding{
		Summary: "fallback",
		Evidence: []model.Evidence{
			{Marker: "alpha"},
			{Marker: "beta"},
		},
	}
	got := evidenceText(f)
	if got != "alpha beta" {
		t.Errorf("evidenceText = %q, want %q", got, "alpha beta")
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "my summary"}
	got := evidenceText(f)
	if got != "my summary" {
		t.Errorf("evidenceText = %q, want %q", got, "my summary")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("empty analyze returned %d models", len(r.Models))
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "gpt-4"}, Summary: "In simple terms for example first second", Lab: "lab1"},
		{Model: model.ModelRef{Model: "gpt-4"}, Summary: "Put simply to illustrate next finally", Lab: "lab2"},
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("got %d models, want 1", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("model = %q, want gpt-4", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("sample count = %d, want 2", r.Models[0].SampleCount)
	}
	if r.Models[0].LabCount != 2 {
		t.Errorf("lab count = %d, want 2", r.Models[0].LabCount)
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("best model = %q, want gpt-4", r.BestModel)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "clear"}, Summary: "In simple terms for example first second for beginners put simply to illustrate", Lab: "a"},
		{Model: model.ModelRef{Model: "jargony"}, Summary: "The eigenvalue heuristic orthogonal idempotent polymorphism monadic functorial isomorphic approach", Lab: "a"},
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("got %d models, want 2", len(r.Models))
	}
	if r.BestModel != "clear" {
		t.Errorf("best = %q, want clear", r.BestModel)
	}
	if r.WorstModel != "jargony" {
		t.Errorf("worst = %q, want jargony", r.WorstModel)
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "in simple terms", Lab: "x"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("model = %q, want unknown", r.Models[0].Model)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m"}, Summary: "eigenvalue heuristic orthogonal idempotent polymorphism monadic functorial isomorphic", Lab: "a"},
	}
	r := Analyze(findings)
	if r.Models[0].Concerns != 1 {
		t.Errorf("concerns = %d, want 1", r.Models[0].Concerns)
	}
}

func TestAnalyzeSorted(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "low"}, Summary: "eigenvalue heuristic orthogonal idempotent polymorphism", Lab: "a"},
		{Model: model.ModelRef{Model: "high"}, Summary: "in simple terms for example first second for beginners", Lab: "a"},
	}
	r := Analyze(findings)
	if r.Models[0].AccessibilityScore < r.Models[1].AccessibilityScore {
		t.Error("models not sorted descending by score")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Report{}
	out := FormatText(r)
	if !strings.Contains(out, "No accessibility data") {
		t.Error("empty report missing expected message")
	}
}

func TestFormatTextContent(t *testing.T) {
	r := Report{
		Models: []ModelAccessibility{{
			Model: "test", Grade: "B", AccessibilityScore: 82.5,
			SampleCount: 3, LabCount: 2, Concerns: 1,
			Detail: AccessibilityDetail{
				PlainLanguage: 0.6, StructureClarity: 0.5,
				ExampleUse: 0.7, JargonAvoidance: 0.8, AudienceAwareness: 0.4,
				AccessibleCount: 8, JargonCount: 2, ComplexCount: 1,
			},
		}},
		BestModel: "test", WorstModel: "test", MeanScore: 82.5,
	}
	out := FormatText(r)
	for _, want := range []string{"Accessibility", "test", "[B]", "82.5", "plain_language", "jargon_avoidance", "Concerns: 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("FormatText missing %q", want)
		}
	}
}

func TestFormatJSON(t *testing.T) {
	r := Report{
		Models:    []ModelAccessibility{{Model: "m1", AccessibilityScore: 70}},
		BestModel: "m1", WorstModel: "m1", MeanScore: 70,
	}
	out := FormatJSON(r)
	if !strings.Contains(out, `"accessibility_score"`) {
		t.Error("JSON missing accessibility_score key")
	}
	if !strings.Contains(out, `"m1"`) {
		t.Error("JSON missing model name")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "a"}, Summary: "in simple terms for example", Lab: "x"},
		{Model: model.ModelRef{Model: "b"}, Summary: "eigenvalue heuristic orthogonal", Lab: "x"},
	}
	r := Analyze(findings)
	expected := math.Round((r.Models[0].AccessibilityScore+r.Models[1].AccessibilityScore)/2*10) / 10
	if r.MeanScore != expected {
		t.Errorf("mean = %.1f, want %.1f", r.MeanScore, expected)
	}
}
