package codeeval

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     "test-lab",
		Summary: summary,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestSingleModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "func main() {\n  fmt.Println(\"hello\")\n}\n"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", r.Models[0].Model)
	}
}

func TestSecurityDetection(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "query = \"SELECT * FROM users WHERE id = \" + input\neval(user_input)\nos.system(cmd)\n"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.SecurityIssues == 0 {
		t.Error("expected security issues detected")
	}
}

func TestSecurityXSS(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "element.innerHTML = userInput\ndocument.write(data)\n<script>alert(1)</script>\n"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.SecurityIssues == 0 {
		t.Error("expected XSS security issues detected")
	}
}

func TestCredentialDetection(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "password = \"secret123\"\napi_key = \"sk-abc123\"\nsecret = \"my_secret\"\n"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.SecurityIssues == 0 {
		t.Error("expected credential security issues detected")
	}
}

func TestNestingDepth(t *testing.T) {
	code := "if (a) {\n  if (b) {\n    if (c) {\n      if (d) {\n        if (e) {\n          x()\n        }\n      }\n    }\n  }\n}\n"
	findings := []model.Finding{
		makeFinding("gpt-4", code),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.NestingDepth < 5 {
		t.Errorf("expected deep nesting >= 5, got %d", r.Models[0].Detail.NestingDepth)
	}
}

func TestLongLines(t *testing.T) {
	longLine := strings.Repeat("x", 150)
	findings := []model.Finding{
		makeFinding("gpt-4", longLine+"\nshort\n"+longLine+"\n"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.LongLines < 2 {
		t.Errorf("expected >= 2 long lines, got %d", r.Models[0].Detail.LongLines)
	}
}

func TestTodoCount(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "// TODO: fix this\n// FIXME: broken\n// HACK: workaround\ncode()\n"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.TodoCount < 3 {
		t.Errorf("expected >= 3 todos, got %d", r.Models[0].Detail.TodoCount)
	}
}

func TestDocumentation(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "// Function validates input\n// @param input string\n// @returns bool\nfunc validate(input string) bool {\n  return true\n}\n"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.DocScore == 0 {
		t.Error("expected positive doc score for documented code")
	}
}

func TestComplexCode(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "if (a) {\n} else if (b) {\n  for (i := 0; i < n; i++) {\n    switch (x) {\n      case 1: try { f() } catch (e) {}\n    }\n  }\n}\nwhile (true) { if (x && y || z) { break } }\n"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.ComplexityScore >= 90 {
		t.Errorf("expected lower complexity score for complex code, got %.1f", r.Models[0].Detail.ComplexityScore)
	}
}

func TestSimpleCode(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "x = 1\ny = 2\nresult = x + y\n"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.ComplexityScore < 50 {
		t.Errorf("expected high complexity score for simple code, got %.1f", r.Models[0].Detail.ComplexityScore)
	}
}

func TestBraceMismatch(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "func main() {\n  if true {\n    x()\n"),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.SyntaxScore >= 85 {
		t.Errorf("expected lower syntax score for brace mismatch, got %.1f", r.Models[0].Detail.SyntaxScore)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "func main() {\n  fmt.Println(\"hello\")\n}\n"),
		makeFinding("claude", "def main():\n  print('hello')\n"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" {
		t.Error("expected best model set")
	}
}

func TestGrading(t *testing.T) {
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
		g := scoreToGrade(tt.score)
		if g != tt.grade {
			t.Errorf("score %.0f: expected %s, got %s", tt.score, tt.grade, g)
		}
	}
}

func TestScoreBounded(t *testing.T) {
	bad := strings.Repeat("eval(x) os.system(cmd) SELECT * FROM users 1=1 <script> ", 20)
	findings := []model.Finding{
		makeFinding("gpt-4", bad),
	}
	r := Analyze(findings)
	if r.Models[0].CodeScore < 0 || r.Models[0].CodeScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].CodeScore)
	}
}

func TestUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{}, Lab: "test-lab", Summary: "code()"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestLabCounting(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "gpt-4"}, Lab: "lab-1", Summary: "code()"},
		{Model: model.ModelRef{Model: "gpt-4"}, Lab: "lab-2", Summary: "code()"},
		{Model: model.ModelRef{Model: "gpt-4"}, Lab: "lab-1", Summary: "code()"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
	if r.Models[0].SampleCount != 3 {
		t.Errorf("expected 3 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestFormatTextNoData(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "Code Evaluation") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "No code evaluation data") {
		t.Error("expected no-data message")
	}
}

func TestFormatTextWithData(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "func main() {\n  // entry point\n  fmt.Println(\"hello\")\n}\n"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name in text output")
	}
	if !strings.Contains(text, "syntax_score") {
		t.Error("expected detail metrics in text output")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "func main() {\n  fmt.Println(\"hello\")\n}\n"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Error("expected 'models' key in JSON")
	}
}

func TestFormatJSONRoundtrip(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if data["mean_score"].(float64) != 0 {
		t.Errorf("expected mean_score 0, got %v", data["mean_score"])
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "func main() {\n  fmt.Println(\"hello\")\n}\n"),
		makeFinding("claude", "x = 1"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Errorf("expected positive mean score, got %.1f", r.MeanScore)
	}
}

func TestSortedByScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("bad-model", "eval(x) os.system(cmd) SELECT * FROM"),
		makeFinding("good-model", "// Validates input\n// @param s string\nfunc validate(s string) bool {\n  return len(s) > 0\n}\n"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].CodeScore < r.Models[1].CodeScore {
		t.Error("expected models sorted by descending score")
	}
}
