package depth

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

func TestAnalyzeTextDeep(t *testing.T) {
	text := "Specifically, the root cause is a race condition in `sync.Mutex`. For example, when two goroutines call `handler.Process()` concurrently, one caveat is that the lock scope is too narrow. Looking deeper, the underlying mechanism is the scheduler preempting at the channel send."
	score, detail := AnalyzeText(text)
	if score <= 50 {
		t.Errorf("deep text should score above 50, got %.1f", score)
	}
	if detail.DetailCount == 0 {
		t.Error("expected detail phrases")
	}
	if detail.ExampleCount == 0 {
		t.Error("expected example phrases")
	}
	if detail.CaveatCount == 0 {
		t.Error("expected caveat phrases")
	}
}

func TestAnalyzeTextShallow(t *testing.T) {
	text := "Just do the obvious thing. It's clearly straightforward and easy. Simply fix it, it's trivial. Of course this is a no-brainer, naturally self-explanatory."
	score, detail := AnalyzeText(text)
	if score >= 50 {
		t.Errorf("shallow text should score below 50, got %.1f", score)
	}
	if detail.ShallowCount == 0 {
		t.Error("expected shallow phrases")
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

func TestDetailDetection(t *testing.T) {
	text := "Specifically, the root cause is here. Looking deeper, the underlying mechanism works by examining closely the implementation."
	count := countPhrases(text, detailPhrases)
	if count < 4 {
		t.Errorf("expected at least 4 detail phrases, got %d", count)
	}
}

func TestExampleDetection(t *testing.T) {
	text := "For example, consider the case of a timeout. For instance, as an example, a 500ms deadline. To illustrate, one scenario involves retries."
	count := countPhrases(text, examplePhrases)
	if count < 4 {
		t.Errorf("expected at least 4 example phrases, got %d", count)
	}
}

func TestCaveatDetection(t *testing.T) {
	text := "However, note that an edge case exists. One caveat is the timeout. Be aware that a corner case applies. Keep in mind the limitation."
	count := countPhrases(text, caveatPhrases)
	if count < 4 {
		t.Errorf("expected at least 4 caveat phrases, got %d", count)
	}
}

func TestBreadthDetection(t *testing.T) {
	text := "Another approach is polling. Alternatively, use websockets. A related concept is event sourcing. Beyond this, extending further."
	count := countPhrases(text, breadthPhrases)
	if count < 3 {
		t.Errorf("expected at least 3 breadth phrases, got %d", count)
	}
}

func TestShallowDetection(t *testing.T) {
	text := "Just do it. Simply obvious. Easy and straightforward. Clearly trivial. Of course a no-brainer."
	count := countPhrases(text, shallowPhrases)
	if count < 5 {
		t.Errorf("expected at least 5 shallow phrases, got %d", count)
	}
}

func TestCodeRefDetection(t *testing.T) {
	text := "The `http.Handler` interface calls `server.ListenAndServe()` with func main defined."
	count := countCodeRefs(text)
	if count < 2 {
		t.Errorf("expected at least 2 code refs, got %d", count)
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
	text := "Just do simply obvious easy straightforward clearly trivial of course no-brainer naturally self-explanatory simply easy just obvious"
	score, _ := AnalyzeText(text)
	if score < 0 || score > 100 {
		t.Errorf("score should be clamped to [0,100], got %.1f", score)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "Specifically, the root cause is in `handler.go`. For example, the `Serve()` func panics on nil input. One caveat is the error path."),
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
		mkFinding("gpt-4", "lab1", "Specifically, the root cause is here. For example, `Process()` fails. One caveat is the lock. Looking deeper, the underlying mechanism is broken."),
		mkFinding("claude", "lab1", "Just do the obvious thing. Simply straightforward. Easy fix, clearly trivial."),
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
		mkFinding("", "lab1", "Specifically, the func returns an error."),
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
		mkFinding("gpt-4", "lab1", "Specifically, the func fails here."),
		mkFinding("gpt-4", "lab2", "For example, `Parse()` returns nil."),
		mkFinding("gpt-4", "lab1", "One caveat is the timeout."),
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
		mkFinding("gpt-4", "lab1", "Specifically, the `API` returns errors. For example, 500 on nil."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Response Depth Analysis") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("missing model name")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	text := FormatText(Report{})
	if !strings.Contains(text, "No depth data") {
		t.Error("missing empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", "Specifically, the func works."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "gpt-4") {
		t.Error("expected model in JSON output")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", "Specifically, for example the `handler.Serve()` func has one caveat regarding timeouts."),
		mkFinding("m2", "lab1", "Just do the obvious thing simply."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestCoverageRatio(t *testing.T) {
	_, detail := AnalyzeText("Specifically, the root cause is clear. For example, it works. Just do it simply.")
	if detail.CoverageRatio <= 0 || detail.CoverageRatio >= 1 {
		t.Errorf("coverage ratio should be between 0 and 1 exclusive, got %.2f", detail.CoverageRatio)
	}
}
