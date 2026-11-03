package dialectic

import (
	"math"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(summary string) model.Finding {
	return model.Finding{
		Summary: summary,
		Model:   model.ModelRef{Model: "test-model"},
		Lab:     "lab1",
	}
}

func findingWithModel(summary, modelName string) model.Finding {
	return model.Finding{
		Summary: summary,
		Model:   model.ModelRef{Model: modelName},
		Lab:     "lab1",
	}
}

func findingWithEvidence(summary string, markers ...string) model.Finding {
	f := finding(summary)
	for _, m := range markers {
		f.Evidence = append(f.Evidence, model.Evidence{Marker: m})
	}
	return f
}

func TestCountPhrases(t *testing.T) {
	tests := []struct {
		text    string
		phrases []string
		want    int
	}{
		{"on the other hand this is good however bad", antithesisPhrases, 2},
		{"the thesis is clear and the argument is strong", thesisPhrases, 2},
		{"", thesisPhrases, 0},
		{"no matches here at all", thesisPhrases, 0},
	}
	for _, tt := range tests {
		got := countPhrases(tt.text, tt.phrases)
		if got != tt.want {
			t.Errorf("countPhrases(%q) = %d, want %d", tt.text[:min(30, len(tt.text))], got, tt.want)
		}
	}
}

func TestScoreResponse_EmptyInput(t *testing.T) {
	score, detail := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Errorf("score out of range: %.1f", score)
	}
	if detail.ThesisCount != 0 {
		t.Errorf("expected 0 thesis count for empty, got %d", detail.ThesisCount)
	}
}

func TestScoreResponse_StrongDialectic(t *testing.T) {
	text := "The thesis is that AI is beneficial. On the other hand, critics argue there are risks. " +
		"However, synthesizing these views, a balanced view suggests we need careful governance. " +
		"It depends on context matters and the nuance here is important. " +
		"Therefore, logically follows that we need regulation."
	score, detail := scoreResponse(text, "")
	if score < 50 {
		t.Errorf("strong dialectic should score >50, got %.1f", score)
	}
	if detail.ThesisCount == 0 {
		t.Error("expected thesis phrases detected")
	}
	if detail.AntithesisCount == 0 {
		t.Error("expected antithesis phrases detected")
	}
	if detail.SynthesisCount == 0 {
		t.Error("expected synthesis phrases detected")
	}
}

func TestScoreResponse_WeakDialectic(t *testing.T) {
	text := "Everyone knows this is obvious. Obviously anyone can see that. " +
		"Without a doubt there is no question about it."
	score, _ := scoreResponse(text, "")
	if score > 40 {
		t.Errorf("weak/dogmatic response should score low, got %.1f", score)
	}
}

func TestScoreResponse_OnlyThesis(t *testing.T) {
	text := "The argument is clear. The central argument holds. The thesis is straightforward."
	_, detail := scoreResponse(text, "")
	if detail.ThesisCount == 0 {
		t.Error("expected thesis phrases")
	}
	if detail.AntithesisCount != 0 {
		t.Errorf("expected no antithesis, got %d", detail.AntithesisCount)
	}
}

func TestScoreResponse_OnlyAntithesis(t *testing.T) {
	text := "However there are problems. On the other hand issues arise. Nevertheless critics argue against it."
	_, detail := scoreResponse(text, "")
	if detail.AntithesisCount == 0 {
		t.Error("expected antithesis phrases")
	}
}

func TestScoreResponse_WithEvidence(t *testing.T) {
	score1, _ := scoreResponse("basic text", "")
	score2, _ := scoreResponse("basic text", "the thesis is strong and on the other hand there are counterarguments")
	if score2 <= score1 {
		t.Errorf("evidence with dialectic phrases should raise score: %.1f vs %.1f", score2, score1)
	}
}

func TestScoreResponse_BoundedScore(t *testing.T) {
	long := strings.Repeat("the thesis is on the other hand synthesizing these views therefore ", 100)
	score, _ := scoreResponse(long, "")
	if score > 100 || score < 0 {
		t.Errorf("score out of bounds: %.1f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{95, "A"}, {90, "A"}, {85, "B"}, {80, "B"},
		{75, "C"}, {70, "C"}, {65, "D"}, {60, "D"},
		{55, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestEvidenceText_WithMarkers(t *testing.T) {
	f := findingWithEvidence("summary", "marker1", "marker2")
	got := evidenceText(f)
	if got != "marker1 marker2" {
		t.Errorf("evidenceText = %q, want %q", got, "marker1 marker2")
	}
}

func TestEvidenceText_NoMarkers(t *testing.T) {
	f := finding("fallback summary")
	got := evidenceText(f)
	if got != "fallback summary" {
		t.Errorf("evidenceText = %q, want %q", got, "fallback summary")
	}
}

func TestAnalyze_Empty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected empty report, got %d models", len(r.Models))
	}
}

func TestAnalyze_SingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("The thesis is that AI helps. On the other hand risks exist. Synthesizing these views we see both."),
		finding("The argument is clear. However counterarguments exist. The resolution is nuanced."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
	if r.BestModel != "test-model" {
		t.Errorf("expected best model test-model, got %s", r.BestModel)
	}
}

func TestAnalyze_MultipleModels(t *testing.T) {
	findings := []model.Finding{
		findingWithModel("The thesis is strong. On the other hand there are risks. Synthesizing these views shows balance.", "model-a"),
		findingWithModel("Everyone knows the answer. Obviously it's clear.", "model-b"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ")
	}
}

func TestAnalyze_SortedByScore(t *testing.T) {
	findings := []model.Finding{
		findingWithModel("Everyone knows this obviously.", "weak-model"),
		findingWithModel("The thesis is clear. On the other hand critics argue otherwise. Synthesizing these views a balanced view emerges. Therefore the conclusion is nuanced.", "strong-model"),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].DialecticScore < r.Models[1].DialecticScore {
		t.Error("models should be sorted descending by score")
	}
}

func TestAnalyze_UnknownModel(t *testing.T) {
	f := model.Finding{Summary: "test", Lab: "lab1"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown' model name, got %q", r.Models[0].Model)
	}
}

func TestAnalyze_MeanScore(t *testing.T) {
	findings := []model.Finding{
		findingWithModel("The thesis is that AI matters. On the other hand there are issues.", "a"),
		findingWithModel("The thesis is that AI matters. On the other hand there are issues.", "b"),
	}
	r := Analyze(findings)
	expected := (r.Models[0].DialecticScore + r.Models[1].DialecticScore) / 2
	expected = math.Round(expected*10) / 10
	if r.MeanScore != expected {
		t.Errorf("MeanScore = %.1f, want %.1f", r.MeanScore, expected)
	}
}

func TestAnalyze_ConcernsCount(t *testing.T) {
	findings := []model.Finding{
		finding("Just a simple statement with no dialectic."),
		finding("Another flat statement."),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns for responses with no antithesis/synthesis")
	}
}

func TestAnalyze_LabCount(t *testing.T) {
	f1 := model.Finding{Summary: "test", Model: model.ModelRef{Model: "m"}, Lab: "lab1"}
	f2 := model.Finding{Summary: "test", Model: model.ModelRef{Model: "m"}, Lab: "lab2"}
	r := Analyze([]model.Finding{f1, f2})
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestFormatText_Empty(t *testing.T) {
	out := FormatText(Report{})
	if !strings.Contains(out, "No dialectic data") {
		t.Error("expected empty message")
	}
}

func TestFormatText_NonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("The thesis is that testing works. On the other hand it can be fragile."),
	})
	out := FormatText(r)
	if !strings.Contains(out, "Dialectic Reasoning") {
		t.Error("expected header")
	}
	if !strings.Contains(out, "test-model") {
		t.Error("expected model name")
	}
}

func TestFormatJSON_Valid(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("The thesis is clear. On the other hand counterarguments exist."),
	})
	out := FormatJSON(r)
	if !strings.Contains(out, "dialectic_score") {
		t.Error("expected dialectic_score in JSON")
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("expected trailing newline")
	}
}

func TestScoreResponse_NuancePhrases(t *testing.T) {
	text := "It depends on the context. Context matters here. There are trade-offs to consider. The nuance here is important."
	_, detail := scoreResponse(text, "")
	if detail.NuanceAwareness <= 0.2 {
		t.Errorf("expected nuance score > 0.2, got %.2f", detail.NuanceAwareness)
	}
}

func TestScoreResponse_RigorPhrases(t *testing.T) {
	text := "Therefore the conclusion is clear. It follows that the evidence suggests this. Logically follows from the premises."
	_, detail := scoreResponse(text, "")
	if detail.LogicalRigor <= 0.2 {
		t.Errorf("expected rigor score > 0.2, got %.2f", detail.LogicalRigor)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
