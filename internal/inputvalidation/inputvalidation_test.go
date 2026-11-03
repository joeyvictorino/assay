package inputvalidation

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

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestNormalInput(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "Normal text input for testing", "Found XSS vulnerability"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
}

func TestEmptyInputDetection(t *testing.T) {
	cat := classifyInput("")
	if cat != CatEmpty {
		t.Errorf("expected empty, got %s", cat)
	}
}

func TestWhitespaceInputDetection(t *testing.T) {
	cat := classifyInput("   \t  \n  ")
	if cat != CatEmpty {
		t.Errorf("expected empty for whitespace, got %s", cat)
	}
}

func TestLongInputDetection(t *testing.T) {
	long := strings.Repeat("A", 20000)
	cat := classifyInput(long)
	if cat != CatLong {
		t.Errorf("expected long, got %s", cat)
	}
}

func TestNullByteDetection(t *testing.T) {
	cat := classifyInput("test\x00input")
	if cat != CatNullBytes {
		t.Errorf("expected null_bytes, got %s", cat)
	}
}

func TestInjectionDetection(t *testing.T) {
	cat := classifyInput("<script>alert('xss')</script>")
	if cat != CatInjection {
		t.Errorf("expected injection, got %s", cat)
	}
}

func TestSQLInjectionDetection(t *testing.T) {
	cat := classifyInput("admin' or 1=1--")
	if cat != CatInjection {
		t.Errorf("expected injection for SQL, got %s", cat)
	}
}

func TestSpecialCharDetection(t *testing.T) {
	cat := classifyInput("test\x1binput")
	if cat != CatSpecialChar {
		t.Errorf("expected special_char, got %s", cat)
	}
}

func TestNormalClassification(t *testing.T) {
	cat := classifyInput("This is a normal test input")
	if cat != CatNormal {
		t.Errorf("expected normal, got %s", cat)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("model-a", "", "Handled empty input gracefully"),
		mkFinding("model-b", "", "<script>reflected xss</script>"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestHandling == r.WorstHandling && len(r.Models) > 1 {
		t.Error("best and worst should differ when scores differ")
	}
}

func TestInjectionHandlingGood(t *testing.T) {
	summaries := []string{
		"Found potential injection attempt, input sanitized",
		"Invalid input detected and rejected",
	}
	score := checkInjectionHandling(summaries)
	if score < 80 {
		t.Errorf("clean summaries should score high, got %.1f", score)
	}
}

func TestInjectionHandlingBad(t *testing.T) {
	summaries := []string{
		"<script>alert('reflected')</script>",
		"Result: <script>test</script>",
	}
	score := checkInjectionHandling(summaries)
	if score > 20 {
		t.Errorf("reflected injection should score low, got %.1f", score)
	}
}

func TestNullByteHandlingGood(t *testing.T) {
	summaries := []string{"Clean output", "No null bytes"}
	score := checkNullByteHandling(summaries)
	if score != 100 {
		t.Errorf("no null bytes should be 100, got %.1f", score)
	}
}

func TestNullByteHandlingBad(t *testing.T) {
	summaries := []string{"output\x00data", "more\x00nulls"}
	score := checkNullByteHandling(summaries)
	if score != 0 {
		t.Errorf("all null bytes should be 0, got %.1f", score)
	}
}

func TestEmptyModelName(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{}, Lab: "test", Summary: "test"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %q", r.Models[0].Model)
	}
}

func TestGradeAssignment(t *testing.T) {
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
	for _, tc := range tests {
		got := scoreToGrade(tc.score)
		if got != tc.grade {
			t.Errorf("scoreToGrade(%.0f) = %q, want %q", tc.score, got, tc.grade)
		}
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "test input", "test summary"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Input Validation") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "test", "test summary"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	if !strings.Contains(out, `"validation_score"`) {
		t.Error("expected validation_score in JSON output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No input validation data") {
		t.Error("expected empty message")
	}
}

func TestCheckEmptyHandlingEmpty(t *testing.T) {
	score := checkEmptyHandling(nil)
	if score != 100 {
		t.Errorf("no samples should be 100, got %.1f", score)
	}
}

func TestCheckEncodingValid(t *testing.T) {
	summaries := []string{"Valid UTF-8 output", "Another valid string"}
	score := checkEncodingHandling(summaries)
	if score != 100 {
		t.Errorf("valid UTF-8 should be 100, got %.1f", score)
	}
}

func TestPathTraversalDetection(t *testing.T) {
	cat := classifyInput("../../etc/passwd")
	if cat != CatInjection {
		t.Errorf("path traversal should be injection, got %s", cat)
	}
}
