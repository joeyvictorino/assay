package rigor

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, lab, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     lab,
		Summary: summary,
	}
}

func TestAnalyzeTextRigorous(t *testing.T) {
	text := "According to RFC 7231, the server returns status code 429. The results show exactly 15 failures per minute. Therefore, the rate limiter is misconfigured as documented in section 4.2."
	score, detail := AnalyzeText(text)
	if score <= 50 {
		t.Errorf("rigorous text should score above 50, got %.1f", score)
	}
	if detail.EvidenceCount == 0 {
		t.Error("expected evidence phrases")
	}
	if detail.LogicCount == 0 {
		t.Error("expected logic phrases")
	}
	if detail.CitationCount == 0 {
		t.Error("expected citation phrases")
	}
}

func TestAnalyzeTextHandwavy(t *testing.T) {
	text := "I think this probably could be an issue. It seems like something might be wrong somehow. I guess we should sort of look into it. I assume it's kind of broken. I believe it should be fine presumably."
	score, detail := AnalyzeText(text)
	if score >= 50 {
		t.Errorf("handwavy text should score below 50, got %.1f", score)
	}
	if detail.HandwaveCount == 0 {
		t.Error("expected handwave phrases")
	}
}

func TestAnalyzeTextEmpty(t *testing.T) {
	score, detail := AnalyzeText("")
	if score != 50.0 {
		t.Errorf("empty text should score 50, got %.1f", score)
	}
	if detail.WordCount != 0 {
		t.Errorf("empty text should have 0 words, got %d", detail.WordCount)
	}
}

func TestEvidenceDetection(t *testing.T) {
	text := "According to the logs, evidence suggests the crash. The results show a clear pattern. Analysis reveals root cause."
	count := countPhrases(text, evidencePhrases)
	if count < 3 {
		t.Errorf("expected at least 3 evidence phrases, got %d", count)
	}
}

func TestLogicDetection(t *testing.T) {
	text := "Therefore this is the cause. Because the timeout expired, consequently the request failed. Since the cache is cold, it follows that latency increases."
	count := countPhrases(text, logicPhrases)
	if count < 4 {
		t.Errorf("expected at least 4 logic phrases, got %d", count)
	}
}

func TestQuantifierDetection(t *testing.T) {
	text := "Exactly 42 requests per second with approximately 5ms latency. At least 100 connections on average."
	count := countPhrases(text, quantifierPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 quantifier phrases, got %d", count)
	}
}

func TestCitationDetection(t *testing.T) {
	text := "Per the OWASP top 10, as defined in RFC 9110, section 15.5 covers this. The NIST specification requires it."
	count := countPhrases(text, citationPhrases)
	if count < 4 {
		t.Errorf("expected at least 4 citation phrases, got %d", count)
	}
}

func TestHandwaveDetection(t *testing.T) {
	text := "I think it might be broken. Probably should be fine. It seems sort of okay. I assume this could be the issue."
	count := countPhrases(text, handwavePhrases)
	if count < 4 {
		t.Errorf("expected at least 4 handwave phrases, got %d", count)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {40, "F"},
	}
	for _, tt := range tests {
		if got := scoreToGrade(tt.score); got != tt.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestScoreClamping(t *testing.T) {
	handwavy := "probably might be could be i think it seems sort of kind of " +
		"somehow i guess i assume i believe presumably should be fine most likely"
	score, _ := AnalyzeText(handwavy)
	if score < 0 || score > 100 {
		t.Errorf("score should be clamped to [0,100], got %.1f", score)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "According to RFC 7231, the API returns exactly 3 error codes. Therefore the handler is correct."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("expected best model gpt-4, got %s", r.BestModel)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "According to the specification, exactly 10 requests per second are allowed. Therefore, the limiter works as defined in section 4."),
		mkFinding("claude", "lab1", "I think it might be broken somehow. It seems sort of wrong. Probably should be fine."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ with different quality texts")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("", "lab1", "The results show exactly 7 failures per minute."),
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models, got %d", len(r.Models))
	}
}

func TestLabCounting(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "Therefore, exactly 10 items."),
		mkFinding("gpt-4", "lab2", "According to the spec, 20 records."),
		mkFinding("gpt-4", "lab1", "The results show 5 entries."),
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
	if r.Models[0].SampleCount != 3 {
		t.Errorf("expected 3 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "According to the spec, exactly 3 endpoints return errors."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Analytical Rigor Analysis") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No rigor data") {
		t.Error("missing empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "Therefore the API is correct."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "gpt-4") {
		t.Error("expected model in JSON output")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", "According to RFC 9110, exactly 10 errors per second."),
		mkFinding("m2", "lab1", "I think it might be broken."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestEvidenceRatio(t *testing.T) {
	_, detail := AnalyzeText("According to the data, the results show the issue. I think maybe it could be a problem.")
	if detail.EvidenceRatio <= 0 || detail.EvidenceRatio >= 1 {
		t.Errorf("evidence ratio should be between 0 and 1 exclusive, got %.2f", detail.EvidenceRatio)
	}
}
