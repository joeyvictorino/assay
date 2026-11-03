package anticipation

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

func TestAnalyzeTextAnticipatory(t *testing.T) {
	text := "Note that this only works on Linux. However, if you're on macOS, you could also use brew. If this doesn't work, check that your PATH is set. Going forward, consider using a version manager."
	score, detail := AnalyzeText(text)
	if score <= 50 {
		t.Errorf("anticipatory text should score above 50, got %.1f", score)
	}
	if detail.WarningCount == 0 {
		t.Error("expected warning phrases")
	}
	if detail.EdgeCaseCount == 0 {
		t.Error("expected edge case phrases")
	}
}

func TestAnalyzeTextMinimal(t *testing.T) {
	text := "Run the command to install the package."
	score, _ := AnalyzeText(text)
	if score > 60 {
		t.Errorf("minimal text should score near baseline, got %.1f", score)
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
	count := countPhrases("Note that this is important: be aware of the limitations. Keep in mind the constraints.", warningPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 warning phrases, got %d", count)
	}
}

func TestEdgeCaseDetection(t *testing.T) {
	count := countPhrases("However, this won't work unless you have access. If you see an error, otherwise try the fallback. In some cases, a different approach is needed.", edgeCasePhrases)
	if count < 3 {
		t.Errorf("expected at least 3 edge case phrases, got %d", count)
	}
}

func TestAlternativeDetection(t *testing.T) {
	count := countPhrases("Alternatively, you could use Docker. Another option is to compile from source. You could also try the prebuilt binary.", alternativePhrases)
	if count < 3 {
		t.Errorf("expected at least 3 alternative phrases, got %d", count)
	}
}

func TestTroubleshootDetection(t *testing.T) {
	count := countPhrases("If this doesn't work, check the logs. Common issues include missing deps. If you encounter problems, make sure that node is installed.", troubleshootPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 troubleshoot phrases, got %d", count)
	}
}

func TestFutureDetection(t *testing.T) {
	count := countPhrases("In the future, consider using TypeScript. Going forward, you may want to add CI. For future reference, keep the docs updated.", futurePhrases)
	if count < 3 {
		t.Errorf("expected at least 3 future phrases, got %d", count)
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
	rich := "Note that this is important. However, unless you do X, it fails. Alternatively, try Y. " +
		"If this doesn't work, check Z. Going forward, consider A. Be aware of B. In some cases, C. " +
		"Another option is D. If you encounter errors, make sure that E. In the future, plan for F."
	score, _ := AnalyzeText(rich)
	if score > 100 {
		t.Errorf("score should not exceed 100, got %.1f", score)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "Note that this requires Python 3.8. However, if you're on 3.7, use the backport."),
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
		mkFinding("anticipating", "lab1", "Note that this is fragile. However, there's a workaround. Alternatively, use Docker. If this doesn't work, check the FAQ."),
		mkFinding("basic", "lab1", "Install the package and run it."),
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
		{Model: model.ModelRef{}, Lab: "lab1", Summary: "Note that this may vary."},
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
		mkFinding("m1", "lab-a", "Note that this is important."),
		mkFinding("m1", "lab-b", "However, there are exceptions."),
		mkFinding("m1", "lab-a", "Alternatively, use a different tool."),
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
		mkFinding("test-model", "lab1", "Note that this is important. However, there are edge cases."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Anticipation") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No anticipation data") {
		t.Error("missing empty message")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", "Note that this is important. However, there are edge cases."),
		mkFinding("m2", "lab1", "Run the command."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestAnticipationRatio(t *testing.T) {
	_, detail := AnalyzeText("Note that this is important. However, unless you do X, it won't work.")
	if detail.AnticipationRatio <= 0 {
		t.Errorf("expected positive ratio, got %.2f", detail.AnticipationRatio)
	}
}

func TestDetailAggregation(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", "Note that this has edge cases. However, there are workarounds."),
		mkFinding("m1", "lab2", "Alternatively, try a different method. If this doesn't work, check logs."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.WarningCount < 1 {
		t.Error("expected warning count")
	}
	if r.Models[0].Detail.AlternativeCount < 1 {
		t.Error("expected alternative count")
	}
}
