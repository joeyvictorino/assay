package causality

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(modelName, summary string, markers ...string) model.Finding {
	f := model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Summary: summary,
		Lab:     "test-lab",
	}
	for _, m := range markers {
		f.Evidence = append(f.Evidence, model.Evidence{Marker: m})
	}
	return f
}

func TestCountPhrases(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		phrases []string
		want    int
	}{
		{"empty text", "", causalPhrases, 0},
		{"no match", "hello world", causalPhrases, 0},
		{"single match", "this is because of the issue", causalPhrases, 1},
		{"multiple matches", "because it leads to failure and results in errors", causalPhrases, 3},
		{"case insensitive", "BECAUSE it LEADS TO problems", causalPhrases, 2},
		{"repeated phrase", "because because because", causalPhrases, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countPhrases(tt.text, tt.phrases)
			if got != tt.want {
				t.Errorf("countPhrases() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestScoreResponseStrongCausal(t *testing.T) {
	text := "The vulnerability exists because the input validation is missing. " +
		"This leads to SQL injection. The mechanism is that user input flows directly " +
		"into the query. First the attacker crafts input, then the server executes it. " +
		"If instead the input were sanitized, the attack would fail. " +
		"However, correlation does not imply causation here."
	score, detail := scoreResponse(text, "")
	if score < 60 {
		t.Errorf("strong causal text scored only %.1f, want >= 60", score)
	}
	if detail.CausalCount == 0 {
		t.Error("expected non-zero causal count")
	}
	if detail.SpeculativeCount != 0 {
		t.Errorf("expected zero speculative count, got %d", detail.SpeculativeCount)
	}
}

func TestScoreResponseSpeculative(t *testing.T) {
	text := "It is possible that this might cause an issue. " +
		"We speculate the connection is uncertain. " +
		"One theory is that it could potentially lead to problems. " +
		"No definitive link has been established."
	score, detail := scoreResponse(text, "")
	if detail.SpeculativeCount == 0 {
		t.Error("expected speculative phrases to be detected")
	}
	if score > 60 {
		t.Errorf("speculative text scored %.1f, want <= 60", score)
	}
}

func TestScoreResponseUnsupported(t *testing.T) {
	text := "Obviously causes the problem. Clearly the reason is undeniable. " +
		"The only explanation proves that it is beyond question."
	score, detail := scoreResponse(text, "")
	if detail.UnsupportedCount == 0 {
		t.Error("expected unsupported phrases to be detected")
	}
	if score > 50 {
		t.Errorf("unsupported text scored %.1f, want <= 50", score)
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, detail := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Errorf("empty response score %.1f out of range", score)
	}
	if detail.CausalCount != 0 {
		t.Errorf("expected zero causal count for empty, got %d", detail.CausalCount)
	}
}

func TestScoreResponseWithEvidence(t *testing.T) {
	response := "The issue exists"
	evidence := "because the root cause triggers the vulnerability first then next"
	score, _ := scoreResponse(response, evidence)
	scoreNoEvidence, _ := scoreResponse(response, "")
	if score <= scoreNoEvidence {
		t.Errorf("evidence should improve score: with=%.1f, without=%.1f", score, scoreNoEvidence)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {90, "A"}, {85, "B"}, {80, "B"},
		{75, "C"}, {70, "C"}, {65, "D"}, {60, "D"},
		{55, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestScoreClampedTo100(t *testing.T) {
	text := strings.Repeat("because leads to results in the root cause triggers the mechanism is step by step if instead without this first then next however confounding factor ", 50)
	score, _ := scoreResponse(text, "")
	if score > 100 {
		t.Errorf("score %.1f exceeds 100", score)
	}
}

func TestScoreClampedTo0(t *testing.T) {
	text := strings.Repeat("obviously causes clearly the reason must be caused by proves that definitively shows the only explanation irrefutable evidence beyond question ", 20)
	score, _ := scoreResponse(text, "")
	if score < 0 {
		t.Errorf("score %.1f below 0", score)
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{Summary: "test summary"}
	if evidenceText(f) != "test summary" {
		t.Error("expected summary as fallback")
	}
	f.Evidence = []model.Evidence{{Marker: "marker1"}, {Marker: "marker2"}}
	got := evidenceText(f)
	if got != "marker1 marker2" {
		t.Errorf("evidenceText() = %q, want %q", got, "marker1 marker2")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected empty report for nil findings")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "The vulnerability exists because input is not sanitized, leads to injection"),
		finding("gpt-4", "This happens because of missing auth. The mechanism is that tokens are not checked"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
	if r.BestModel != "gpt-4" || r.WorstModel != "gpt-4" {
		t.Error("best and worst should both be gpt-4")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("model-a", "Because the root cause leads to failure. The mechanism is clear. First then next."),
		finding("model-b", "It might cause something. We speculate. One theory is unclear."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "because of the root cause", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown model, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "because causes"),
		{Model: model.ModelRef{Model: "m1"}, Summary: "leads to", Lab: "lab2"},
	}
	findings[0].Lab = "lab1"
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeSortedByScore(t *testing.T) {
	findings := []model.Finding{
		finding("bad-model", "it might cause. we speculate. one theory is. no definitive link."),
		finding("good-model", "because the root cause leads to this. the mechanism is clear. first then next. if instead it would not happen."),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].CausalityScore < r.Models[1].CausalityScore {
		t.Error("models should be sorted descending by score")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	s := FormatText(Report{})
	if !strings.Contains(s, "No causality data") {
		t.Error("expected no-data message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "because leads to results in the mechanism is"),
	})
	s := FormatText(r)
	if !strings.Contains(s, "Causal Reasoning Quality") {
		t.Error("expected header")
	}
	if !strings.Contains(s, "test-model") {
		t.Error("expected model name")
	}
	if !strings.Contains(s, "causal_chain_score") {
		t.Error("expected detail field")
	}
}

func TestFormatJSONValid(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "because of root cause"),
	})
	s := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("expected 1 model in JSON, got %d", len(parsed.Models))
	}
}

func TestFormatJSONEmpty(t *testing.T) {
	s := FormatJSON(Report{})
	var parsed Report
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "because the root cause leads to problems"),
		finding("m2", "because the mechanism is clear"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestConcernsCount(t *testing.T) {
	text := "might cause. could potentially lead to. it is possible that. " +
		"one theory is. we speculate. hypothetically. it remains unclear. " +
		"the connection is uncertain. no definitive link."
	findings := []model.Finding{finding("m1", text)}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns for highly speculative text")
	}
}
