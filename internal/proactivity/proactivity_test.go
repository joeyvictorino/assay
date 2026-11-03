package proactivity

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

func TestAnalyzeTextProactive(t *testing.T) {
	text := "Be careful with this endpoint. To prevent SQL injection, always check user input. Alternatively, consider using parameterized queries. Going forward, plan for input validation at every boundary."
	score, detail := AnalyzeText(text)
	if score <= 50 {
		t.Errorf("proactive text should score above 50, got %.1f", score)
	}
	if detail.WarningCount == 0 {
		t.Error("expected warning phrases")
	}
	if detail.PreventionCount == 0 {
		t.Error("expected prevention phrases")
	}
	if detail.AlternativeCount == 0 {
		t.Error("expected alternative phrases")
	}
}

func TestAnalyzeTextReactive(t *testing.T) {
	text := "If it breaks just restart the server. When it fails check the logs. After the crash you can try rebooting. Once the error occurs, the fix is to redeploy. As a workaround, to recover from the failure restart."
	score, detail := AnalyzeText(text)
	if score >= 50 {
		t.Errorf("reactive text should score below 50, got %.1f", score)
	}
	if detail.ReactiveCount == 0 {
		t.Error("expected reactive phrases")
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

func TestWarningDetection(t *testing.T) {
	text := "Be careful with this API. Watch out for rate limits. Note that the endpoint is deprecated. Be aware of breaking changes."
	count := countPhrases(text, warningPhrases)
	if count < 4 {
		t.Errorf("expected at least 4 warning phrases, got %d", count)
	}
}

func TestPreventionDetection(t *testing.T) {
	text := "To prevent data loss, ensure that backups exist. Before you deploy, always check the config. Never use hardcoded credentials."
	count := countPhrases(text, preventionPhrases)
	if count < 4 {
		t.Errorf("expected at least 4 prevention phrases, got %d", count)
	}
}

func TestAlternativeDetection(t *testing.T) {
	text := "Alternatively, use a CDN. Another approach is caching. Consider using connection pooling. A safer option is to batch requests."
	count := countPhrases(text, alternativePhrases)
	if count < 3 {
		t.Errorf("expected at least 3 alternative phrases, got %d", count)
	}
}

func TestFutureLookDetection(t *testing.T) {
	text := "In the future, plan for horizontal scaling. When you scale to production, prepare for high traffic. As your system grows, anticipate bottlenecks."
	count := countPhrases(text, futureLookPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 future-look phrases, got %d", count)
	}
}

func TestReactiveDetection(t *testing.T) {
	text := "If it breaks, restart. When it fails, check logs. After the crash, redeploy. Once the error occurs, debug. To fix the issue, patch it."
	count := countPhrases(text, reactivePhrases)
	if count < 4 {
		t.Errorf("expected at least 4 reactive phrases, got %d", count)
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
	reactive := "if it breaks when it fails after the crash once the error occurs " +
		"after the damage to fix the issue to resolve the problem as a workaround " +
		"the fix is to to recover from"
	score, _ := AnalyzeText(reactive)
	if score < 0 || score > 100 {
		t.Errorf("score should be clamped to [0,100], got %.1f", score)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "Be careful with this. To prevent issues, ensure that validation is in place. Going forward, plan for scaling."),
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
		mkFinding("gpt-4", "lab1", "Be careful! To prevent issues, always check first. Alternatively, use a safer approach. Going forward, prepare for growth."),
		mkFinding("claude", "lab1", "If it breaks, restart it. When it fails, check the logs. After the crash, redeploy."),
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
		mkFinding("", "lab1", "Be careful, note that this endpoint is rate limited."),
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
		mkFinding("gpt-4", "lab1", "Be careful with this API."),
		mkFinding("gpt-4", "lab2", "To prevent data loss, ensure backups."),
		mkFinding("gpt-4", "lab1", "Alternatively, use caching."),
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
		mkFinding("gpt-4", "lab1", "Be careful with the 3 exposed endpoints."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Proactivity Analysis") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No proactivity data") {
		t.Error("missing empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "To prevent issues, always validate input."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "gpt-4") {
		t.Error("expected model in JSON output")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", "Be careful! To prevent issues, always check. Going forward, plan for it."),
		mkFinding("m2", "lab1", "If it breaks, restart it. After the crash, redeploy."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestProactiveRatio(t *testing.T) {
	_, detail := AnalyzeText("Be careful with this. To prevent issues, validate input. If it breaks, restart the server.")
	if detail.ProactiveRatio <= 0 || detail.ProactiveRatio >= 1 {
		t.Errorf("proactive ratio should be between 0 and 1 exclusive, got %.2f", detail.ProactiveRatio)
	}
}
