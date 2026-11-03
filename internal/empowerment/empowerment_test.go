package empowerment

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

func TestAnalyzeTextEmpowering(t *testing.T) {
	text := "You can fix this by running `npm install`. First, open the terminal. Then, navigate to the project directory. Use the `--save` flag to persist. See the documentation for more details."
	score, detail := AnalyzeText(text)
	if score <= 50 {
		t.Errorf("empowering text should score above 50, got %.1f", score)
	}
	if detail.ActionableCount == 0 {
		t.Error("expected actionable phrases")
	}
	if detail.StepCount == 0 {
		t.Error("expected step phrases")
	}
	if detail.CodeRefCount == 0 {
		t.Error("expected code references")
	}
}

func TestAnalyzeTextDependency(t *testing.T) {
	text := "Contact support to get this resolved. You'll need help from your admin. Reach out to your team lead for access. Submit a ticket and have someone look into it."
	score, detail := AnalyzeText(text)
	if score >= 50 {
		t.Errorf("dependency-heavy text should score below 50, got %.1f", score)
	}
	if detail.DependencyCount == 0 {
		t.Error("expected dependency phrases")
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

func TestActionableDetection(t *testing.T) {
	count := countPhrases("You can try running the command. Use the API to create a new resource.", actionablePhrases)
	if count < 3 {
		t.Errorf("expected at least 3 actionable phrases, got %d", count)
	}
}

func TestStepDetection(t *testing.T) {
	count := countPhrases("First, install the package. Then, configure the settings. Finally, run the tests.", stepPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 step phrases, got %d", count)
	}
}

func TestCodeRefDetection(t *testing.T) {
	count := countCodeRefs("Run `npm install` and then `npm start` to launch the `dev` server")
	if count != 3 {
		t.Errorf("expected 3 code refs, got %d", count)
	}
}

func TestSelfHelpDetection(t *testing.T) {
	count := countPhrases("Check the documentation for more details. The official guide has a tutorial on this topic.", selfHelpPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 self-help phrases, got %d", count)
	}
}

func TestDependencyDetection(t *testing.T) {
	count := countPhrases("Contact support for help. Ask your admin to grant access. Submit a ticket.", dependencyPhrases)
	if count >= 3 {
		return
	}
	t.Errorf("expected at least 3 dependency phrases, got %d", count)
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
	score1, _ := AnalyzeText("You can try running `a`. Use the `b` command. First, install `c`. Then, configure `d`. See the documentation and the official guide and the tutorial for more details.")
	if score1 > 100 {
		t.Errorf("score should not exceed 100, got %.1f", score1)
	}

	score2, _ := AnalyzeText("Contact support. You'll need help from your admin. Reach out to your team lead. Have someone submit a ticket. Open a support case. Get assistance from your manager. Request access from support.")
	if score2 < 0 {
		t.Errorf("score should not go below 0, got %.1f", score2)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "You can fix this by running `npm install`. First, open the terminal."),
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
		mkFinding("helpful", "lab1", "You can fix this. First, run `npm install`. Then, configure the settings. Use the documentation for reference."),
		mkFinding("unhelpful", "lab1", "Contact support. You'll need help from your admin. Reach out to your team lead."),
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
		{Model: model.ModelRef{}, Lab: "lab1", Summary: "You can try this approach."},
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
		mkFinding("m1", "lab-a", "You can use the API."),
		mkFinding("m1", "lab-b", "Try running the tests."),
		mkFinding("m1", "lab-a", "First, install the package."),
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
		mkFinding("test-model", "lab1", "You can try running `npm test`."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Empowerment") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No empowerment data") {
		t.Error("missing empty message")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", "You can use the `api` command. First, configure it."),
		mkFinding("m2", "lab1", "Contact support for help."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestEmpowermentRatio(t *testing.T) {
	_, detail := AnalyzeText("You can try running `cmd`. Contact support if needed.")
	if detail.EmpowermentRatio <= 0 {
		t.Errorf("expected positive ratio, got %.2f", detail.EmpowermentRatio)
	}
}

func TestDetailAggregation(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", "Contact support for help. You'll need help from your admin."),
		mkFinding("m1", "lab2", "You can try running `npm test`. First, install dependencies."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.DependencyCount < 1 {
		t.Errorf("expected dependencies, got %d", r.Models[0].Detail.DependencyCount)
	}
	if r.Models[0].Detail.ActionableCount < 1 {
		t.Errorf("expected actionable, got %d", r.Models[0].Detail.ActionableCount)
	}
}
