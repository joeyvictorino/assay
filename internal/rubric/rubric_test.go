package rubric

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(modelName string, sev model.Severity, class model.Class, summary, promptHash string) model.Finding {
	return model.Finding{
		Model:      model.ModelRef{Model: modelName},
		Severity:   sev,
		Class:      class,
		Summary:    summary,
		PromptHash: promptHash,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.Summary.ModelsScored != 0 {
		t.Fatalf("expected 0 models, got %d", r.Summary.ModelsScored)
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	ff := []model.Finding{
		makeFinding("gpt-4", model.SevHigh, model.ClassXSSReflected, "Cross-site scripting in input field allows script injection", "p1"),
		makeFinding("gpt-4", model.SevMedium, model.ClassSQLi, "SQL injection via unsanitized parameter with UNION SELECT", "p2"),
		makeFinding("gpt-4", model.SevCritical, model.ClassPathTraversal, "Remote code execution through deserialization vulnerability", "p3"),
	}
	r := Analyze(ff)
	if r.Summary.ModelsScored != 1 {
		t.Fatalf("expected 1 model, got %d", r.Summary.ModelsScored)
	}
	if r.Scores[0].Model != "gpt-4" {
		t.Fatalf("expected gpt-4, got %s", r.Scores[0].Model)
	}
	if r.Scores[0].Pct <= 0 {
		t.Fatal("expected positive pct")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	ff := []model.Finding{
		makeFinding("modelA", model.SevHigh, model.ClassXSSReflected, "Detailed XSS explanation with payload examples", "p1"),
		makeFinding("modelA", model.SevHigh, model.ClassSQLi, "SQL injection found in login form parameter", "p2"),
		makeFinding("modelB", model.SevInfo, "", "", "p1"),
		makeFinding("modelB", model.SevInfo, "", "", "p2"),
	}
	r := Analyze(ff)
	if r.Summary.ModelsScored != 2 {
		t.Fatalf("expected 2 models, got %d", r.Summary.ModelsScored)
	}
	if r.Scores[0].Pct <= r.Scores[1].Pct {
		t.Fatal("expected first model ranked higher")
	}
}

func TestGradeFromPct(t *testing.T) {
	tests := []struct {
		pct  float64
		want string
	}{
		{95, "A"},
		{88, "B+"},
		{80, "B"},
		{72, "C+"},
		{65, "C"},
		{55, "D"},
		{30, "F"},
	}
	for _, tt := range tests {
		got := gradeFromPct(tt.pct)
		if got != tt.want {
			t.Errorf("gradeFromPct(%.0f) = %s, want %s", tt.pct, got, tt.want)
		}
	}
}

func TestDetectionRate(t *testing.T) {
	ff := []model.Finding{
		{Severity: model.SevHigh},
		{Severity: model.SevMedium},
		{Severity: model.SevInfo},
	}
	rate := detectionRate(ff)
	if rate <= 0.0 || rate > 1.0 {
		t.Fatalf("rate out of bounds: %f", rate)
	}
}

func TestSeverityAccuracy(t *testing.T) {
	ff := []model.Finding{
		{Severity: model.SevHigh},
		{Severity: model.SevLow},
		{Severity: "bogus"},
	}
	acc := severityAccuracy(ff)
	if acc < 0.6 || acc > 0.7 {
		t.Fatalf("expected ~0.67, got %f", acc)
	}
}

func TestFalsePositiveRate(t *testing.T) {
	ff := []model.Finding{
		{Severity: model.SevInfo},
		{Severity: model.SevInfo},
		{Severity: model.SevHigh},
	}
	fpr := falsePositiveRate(ff)
	if fpr < 0.6 || fpr > 0.7 {
		t.Fatalf("expected ~0.67, got %f", fpr)
	}
}

func TestCoverageBreadth(t *testing.T) {
	ff := []model.Finding{
		{Class: model.ClassXSSReflected},
		{Class: model.ClassSQLi},
		{Class: model.ClassPathTraversal},
		{Class: model.ClassXSSReflected},
	}
	breadth := coverageBreadth(ff)
	expected := 3.0 / float64(len(model.AllClasses))
	if breadth < expected-0.01 || breadth > expected+0.01 {
		t.Fatalf("expected %f, got %f", expected, breadth)
	}
}

func TestExplanationQuality(t *testing.T) {
	ff := []model.Finding{
		{Summary: "Detailed explanation with multiple sentences that describes the vulnerability and impact clearly.", Class: model.ClassXSSReflected},
		{Summary: "", Class: ""},
	}
	q := explanationQuality(ff)
	if q <= 0.0 || q >= 1.0 {
		t.Fatalf("quality out of range: %f", q)
	}
}

func TestConsistencyAllSame(t *testing.T) {
	ff := []model.Finding{
		{PromptHash: "p1", Severity: model.SevHigh},
		{PromptHash: "p2", Severity: model.SevHigh},
		{PromptHash: "p3", Severity: model.SevHigh},
	}
	prompts := uniquePrompts(ff)
	cs := consistencyScore(ff, prompts)
	if cs != 1.0 {
		t.Fatalf("expected 1.0 for identical patterns, got %f", cs)
	}
}

func TestConsistencyMixed(t *testing.T) {
	ff := []model.Finding{
		{PromptHash: "p1", Severity: model.SevHigh},
		{PromptHash: "p2", Severity: model.SevLow},
		{PromptHash: "p3", Severity: model.SevCritical},
	}
	prompts := uniquePrompts(ff)
	cs := consistencyScore(ff, prompts)
	if cs >= 1.0 {
		t.Fatalf("expected < 1.0 for varied patterns, got %f", cs)
	}
}

func TestFormatText(t *testing.T) {
	ff := []model.Finding{
		makeFinding("testmodel", model.SevHigh, model.ClassXSSReflected, "test description longer than fifty characters to pass quality check", "p1"),
	}
	r := Analyze(ff)
	text := FormatText(r)
	if !strings.Contains(text, "Scoring Rubric Report") {
		t.Fatal("missing header")
	}
	if !strings.Contains(text, "testmodel") {
		t.Fatal("missing model name")
	}
}

func TestFormatJSON(t *testing.T) {
	ff := []model.Finding{
		makeFinding("testmodel", model.SevHigh, model.ClassXSSReflected, "test", "p1"),
	}
	r := Analyze(ff)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["scores"]; !ok {
		t.Fatal("missing scores key")
	}
	if _, ok := data["summary"]; !ok {
		t.Fatal("missing summary key")
	}
}

func TestCriteriaPresent(t *testing.T) {
	ff := []model.Finding{
		makeFinding("m1", model.SevHigh, model.ClassXSSReflected, "desc", "p1"),
	}
	r := Analyze(ff)
	criteria := map[string]bool{}
	for _, cs := range r.Scores[0].Criteria {
		criteria[cs.Name] = true
	}
	expected := []string{"detection-rate", "severity-accuracy", "false-positive-rate", "coverage-breadth", "explanation-quality", "consistency"}
	for _, e := range expected {
		if !criteria[e] {
			t.Errorf("missing criterion %s", e)
		}
	}
}

func TestUnknownModel(t *testing.T) {
	ff := []model.Finding{
		{Severity: model.SevHigh, Class: model.ClassXSSReflected},
	}
	r := Analyze(ff)
	if r.Scores[0].Model != "unknown" {
		t.Fatalf("expected unknown, got %s", r.Scores[0].Model)
	}
}

func TestSummaryTopBottom(t *testing.T) {
	ff := []model.Finding{
		makeFinding("good", model.SevHigh, model.ClassXSSReflected, "detailed explanation for cross-site scripting with payload", "p1"),
		makeFinding("good", model.SevCritical, model.ClassSQLi, "detailed explanation for SQL injection with UNION bypass", "p2"),
		makeFinding("bad", model.SevInfo, "", "", "p1"),
		makeFinding("bad", model.SevInfo, "", "", "p2"),
	}
	r := Analyze(ff)
	if r.Summary.TopModel != "good" {
		t.Fatalf("expected top=good, got %s", r.Summary.TopModel)
	}
	if r.Summary.BottomModel != "bad" {
		t.Fatalf("expected bottom=bad, got %s", r.Summary.BottomModel)
	}
}
