package contextrecall

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

func TestCountPhrases(t *testing.T) {
	text := "as mentioned earlier in the conversation you said this was important"
	if n := countPhrases(text, recallSignals); n < 1 {
		t.Fatalf("expected at least 1 recall signal match, got %d", n)
	}
	if n := countPhrases(text, contextRetention); n < 1 {
		t.Fatalf("expected at least 1 retention match, got %d", n)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	if n := countPhrases("", recallSignals); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "AS MENTIONED you SAID this in YOUR EARLIER message"
	if n := countPhrases(text, recallSignals); n < 1 {
		t.Fatalf("expected at least 1 match, got %d", n)
	}
}

func TestScoreResponseBasic(t *testing.T) {
	score, detail := scoreResponse("as mentioned in the conversation you said this earlier, referring to the context", "")
	if score < 30 {
		t.Fatalf("expected score >= 30, got %.1f", score)
	}
	if detail.RecallSignalCount < 0 {
		t.Fatal("recall signal count should be non-negative")
	}
}

func TestScoreResponseConfusionPenalty(t *testing.T) {
	goodRecall, _ := scoreResponse("as mentioned you said in your earlier message referring to the conversation context", "")
	confused, _ := scoreResponse("i apologize for the confusion i misunderstood i confused the context let me correct my mistake", "")
	if confused >= goodRecall {
		t.Fatalf("confusion should reduce score: good=%.1f confused=%.1f", goodRecall, confused)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	score, _ := scoreResponse(strings.Repeat("as mentioned you said earlier in conversation context thread ", 50), "")
	if score < 0 || score > 100 {
		t.Fatalf("score out of bounds: %.1f", score)
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, _ := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Fatalf("score out of bounds: %.1f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{45, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected empty models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "as mentioned you said in the conversation context earlier"),
		finding("gpt-4", "referring to your earlier message in the discussion thread"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.BestModel != "gpt-4" {
		t.Fatalf("expected best model gpt-4, got %s", r.BestModel)
	}
	if r.Models[0].SampleCount != 2 {
		t.Fatalf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "as mentioned you said earlier in the conversation context thread discussion"),
		finding("claude-3", "i apologize for the confusion i misunderstood i confused mixed up"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].RecallScore < r.Models[1].RecallScore {
		t.Fatal("models should be sorted by score descending")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Lab: "lab1", Summary: "as mentioned in the context"},
	}
	r := Analyze(findings)
	if r.BestModel != "unknown" {
		t.Fatalf("expected unknown model, got %s", r.BestModel)
	}
}

func TestAnalyzeMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "as mentioned you said in the conversation context"),
		finding("m2", "i apologize for the confusion mixed up"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Fatalf("expected positive mean score, got %.1f", r.MeanScore)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "as mentioned"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab2", Summary: "you said"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "i apologize for the confusion i misunderstood i confused that was incorrect let me correct my mistake"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns < 1 {
		t.Fatal("expected at least 1 concern for high confusion")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No context recall data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "as mentioned you said in the earlier conversation context"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Context Recall") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Fatal("expected model name")
	}
	if !strings.Contains(text, "direct_recall") {
		t.Fatal("expected detail field")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "as mentioned in the context you said earlier"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Fatal("expected models key")
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

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "test summary",
		Evidence: []model.Evidence{
			{Marker: "marker1"},
			{Marker: "marker2"},
		},
	}
	text := evidenceText(f)
	if !strings.Contains(text, "marker1") {
		t.Fatal("expected marker1")
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "fallback summary"}
	text := evidenceText(f)
	if text != "fallback summary" {
		t.Fatalf("expected fallback, got %s", text)
	}
}

func TestRecallSignalsDetected(t *testing.T) {
	_, detail := scoreResponse("as mentioned you said in your earlier message referring to", "")
	if detail.DirectRecall <= 0.2 {
		t.Fatalf("expected direct recall above baseline, got %.2f", detail.DirectRecall)
	}
}

func TestRetentionSignalsDetected(t *testing.T) {
	_, detail := scoreResponse("in this conversation thread earlier discussion previous message", "")
	if detail.ContextRetention <= 0.2 {
		t.Fatalf("expected context retention above baseline, got %.2f", detail.ContextRetention)
	}
}

func TestCitationDetected(t *testing.T) {
	_, detail := scoreResponse("you specifically stated your exact words to quote verbatim accurately", "")
	if detail.CitationAccuracy <= 0.2 {
		t.Fatalf("expected citation accuracy above baseline, got %.2f", detail.CitationAccuracy)
	}
}
