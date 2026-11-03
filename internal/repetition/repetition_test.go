package repetition

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
		Lab:     "test-lab",
	}
}

func TestCountPhrases(t *testing.T) {
	text := "as i mentioned earlier, to reiterate the point, as i mentioned again"
	got := countPhrases(text, fillerPhrases)
	if got < 3 {
		t.Errorf("expected >= 3 filler hits, got %d", got)
	}
}

func TestCountPhrasesZero(t *testing.T) {
	text := "a perfectly clean response with no issues"
	got := countPhrases(text, fillerPhrases)
	if got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestCountRepeatedSentences(t *testing.T) {
	text := "This is a test sentence. This is another one. This is a test sentence."
	repeated, total := countRepeatedSentences(text)
	if repeated < 1 {
		t.Errorf("expected at least 1 repeated sentence, got %d (total %d)", repeated, total)
	}
}

func TestCountRepeatedSentencesNone(t *testing.T) {
	text := "First unique sentence. Second unique sentence. Third unique sentence."
	repeated, _ := countRepeatedSentences(text)
	if repeated != 0 {
		t.Errorf("expected 0 repeated, got %d", repeated)
	}
}

func TestUniqueWordRatio(t *testing.T) {
	high := uniqueWordRatio("the quick brown fox jumps over the lazy dog")
	if high < 0.7 {
		t.Errorf("expected high unique ratio, got %.2f", high)
	}
}

func TestUniqueWordRatioLow(t *testing.T) {
	low := uniqueWordRatio("the the the the the the the the")
	if low > 0.2 {
		t.Errorf("expected low unique ratio, got %.2f", low)
	}
}

func TestUniqueWordRatioEmpty(t *testing.T) {
	r := uniqueWordRatio("")
	if r != 1.0 {
		t.Errorf("expected 1.0 for empty, got %.2f", r)
	}
}

func TestScoreResponseClean(t *testing.T) {
	score, detail := scoreResponse("The algorithm uses binary search for efficient lookup.", "")
	if score < 70 {
		t.Errorf("expected clean score >= 70, got %.1f", score)
	}
	if detail.RepeatCount != 0 {
		t.Errorf("expected 0 repeat count, got %d", detail.RepeatCount)
	}
}

func TestScoreResponseRepetitive(t *testing.T) {
	text := "As I mentioned, to reiterate, as I mentioned before, basically essentially it goes without saying needless to say each and every past history"
	score, detail := scoreResponse(text, "")
	if detail.FillerCount == 0 {
		t.Errorf("expected filler count > 0")
	}
	if detail.RepeatCount == 0 {
		t.Errorf("expected repeat count > 0")
	}
	_ = score
}

func TestScoreResponseCircular(t *testing.T) {
	text := "This is because the reason is which means that therefore we can see going back to as mentioned earlier"
	_, detail := scoreResponse(text, "")
	if detail.CircularLogic >= 0.9 {
		t.Errorf("expected circular score < 0.9, got %.2f", detail.CircularLogic)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	score, _ := scoreResponse("test", "")
	if score < 0 || score > 100 {
		t.Errorf("score out of bounds: %.1f", score)
	}

	heavy := strings.Repeat("as i mentioned basically essentially ", 50)
	score2, _ := scoreResponse(heavy, "")
	if score2 < 0 || score2 > 100 {
		t.Errorf("heavy score out of bounds: %.1f", score2)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {50, "F"},
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
		t.Errorf("expected empty models")
	}
	if r.MeanScore != 0 {
		t.Errorf("expected 0 mean score")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "A clear response about algorithms"),
		finding("gpt-4", "Another clear response about data structures"),
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
	if r.BestModel != "gpt-4" {
		t.Errorf("expected best = gpt-4")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("model-a", "Clear precise response"),
		finding("model-b", "As I mentioned, to reiterate, basically essentially"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst model set")
	}
}

func TestAnalyzeSortedDescending(t *testing.T) {
	findings := []model.Finding{
		finding("bad", "As I mentioned, to reiterate, basically each and every past history needless to say"),
		finding("good", "Efficient binary search returns in O(log n) time"),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Skip("not enough models")
	}
	if r.Models[0].RepetitionScore < r.Models[1].RepetitionScore {
		t.Errorf("expected descending sort")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "test response", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test2", Lab: "lab2"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test3", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "As I mentioned before, to reiterate the point, basically each and every end result past history free gift"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns > 0 for repetitive response")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "Repetition") {
		t.Error("expected header")
	}
	if !strings.Contains(text, "No repetition data") {
		t.Error("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{finding("gpt-4", "A clean response")})
	text := FormatText(r)
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name in output")
	}
	if !strings.Contains(text, "phrase_repeat") {
		t.Error("expected detail field")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("gpt-4", "A clean response")})
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("expected 1 model in JSON")
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

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback summary",
		Evidence: []model.Evidence{
			{Marker: "marker one"},
			{Marker: "marker two"},
		},
	}
	got := evidenceText(f)
	if !strings.Contains(got, "marker one") {
		t.Errorf("expected marker, got %s", got)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "the summary"}
	got := evidenceText(f)
	if got != "the summary" {
		t.Errorf("expected summary fallback, got %s", got)
	}
}

func TestGradeEdgeCases(t *testing.T) {
	if scoreToGrade(90) != "A" {
		t.Error("90 should be A")
	}
	if scoreToGrade(89.9) != "B" {
		t.Error("89.9 should be B")
	}
	if scoreToGrade(0) != "F" {
		t.Error("0 should be F")
	}
}
