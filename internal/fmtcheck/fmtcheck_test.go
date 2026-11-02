package fmtcheck

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m, id, summary string) model.Finding {
	return model.Finding{
		ID:         id,
		Model:      model.ModelRef{Model: m},
		PromptHash: "ph-" + id,
		Summary:    summary,
		State:      model.StateTheorized,
		Class:      model.ClassSQLi,
	}
}

func TestDetectExpectedFormat_JSON(t *testing.T) {
	if f := DetectExpectedFormat("return JSON output"); f != FmtJSON {
		t.Fatalf("expected json, got %s", f)
	}
}

func TestDetectExpectedFormat_Markdown(t *testing.T) {
	if f := DetectExpectedFormat("use markdown format"); f != FmtMarkdown {
		t.Fatalf("expected markdown, got %s", f)
	}
}

func TestDetectExpectedFormat_List(t *testing.T) {
	if f := DetectExpectedFormat("enumerate all items"); f != FmtList {
		t.Fatalf("expected list, got %s", f)
	}
}

func TestDetectExpectedFormat_Code(t *testing.T) {
	if f := DetectExpectedFormat("provide code example"); f != FmtCode {
		t.Fatalf("expected code, got %s", f)
	}
}

func TestDetectExpectedFormat_Any(t *testing.T) {
	if f := DetectExpectedFormat("describe the vulnerability"); f != FmtAny {
		t.Fatalf("expected any, got %s", f)
	}
}

func TestScoreCompliance_ValidJSON(t *testing.T) {
	score, issues := ScoreCompliance(`{"key": "value"}`, FmtJSON)
	if score != 1.0 {
		t.Fatalf("expected 1.0, got %f", score)
	}
	if len(issues) != 0 {
		t.Fatal("expected no issues")
	}
}

func TestScoreCompliance_InvalidJSON(t *testing.T) {
	score, issues := ScoreCompliance(`{"key": broken}`, FmtJSON)
	if score >= 1.0 {
		t.Fatal("expected score < 1.0 for invalid JSON")
	}
	if len(issues) == 0 {
		t.Fatal("expected issues")
	}
}

func TestScoreCompliance_NoJSON(t *testing.T) {
	score, _ := ScoreCompliance("just some text", FmtJSON)
	if score > 0.3 {
		t.Fatalf("expected low score for non-JSON, got %f", score)
	}
}

func TestScoreCompliance_Markdown(t *testing.T) {
	md := "# Title\n\nSome text\n\n- item 1\n- item 2"
	score, _ := ScoreCompliance(md, FmtMarkdown)
	if score < 0.8 {
		t.Fatalf("expected high score for good markdown, got %f", score)
	}
}

func TestScoreCompliance_MarkdownNoHeadings(t *testing.T) {
	score, issues := ScoreCompliance("just text", FmtMarkdown)
	if score >= 1.0 {
		t.Fatal("expected imperfect score without headings")
	}
	found := false
	for _, iss := range issues {
		if strings.Contains(iss, "heading") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected heading issue")
	}
}

func TestScoreCompliance_List(t *testing.T) {
	list := "- item 1\n- item 2\n- item 3"
	score, _ := ScoreCompliance(list, FmtList)
	if score < 0.8 {
		t.Fatalf("expected high score for list, got %f", score)
	}
}

func TestScoreCompliance_ListEmpty(t *testing.T) {
	score, issues := ScoreCompliance("no list here", FmtList)
	if score > 0.3 {
		t.Fatalf("expected low score, got %f", score)
	}
	if len(issues) == 0 {
		t.Fatal("expected issues")
	}
}

func TestScoreCompliance_Code(t *testing.T) {
	code := "Here's the fix:\n```python\nprint('hello')\n```"
	score, _ := ScoreCompliance(code, FmtCode)
	if score != 1.0 {
		t.Fatalf("expected 1.0 for proper code block, got %f", score)
	}
}

func TestScoreCompliance_CodeUnclosed(t *testing.T) {
	code := "```python\nprint('hello')"
	score, issues := ScoreCompliance(code, FmtCode)
	if score >= 1.0 {
		t.Fatal("expected imperfect score for unclosed block")
	}
	if len(issues) == 0 {
		t.Fatal("expected issues")
	}
}

func TestScoreCompliance_Empty(t *testing.T) {
	score, issues := ScoreCompliance("", FmtJSON)
	if score != 0.0 {
		t.Fatalf("expected 0.0, got %f", score)
	}
	if len(issues) == 0 {
		t.Fatal("expected issues for empty")
	}
}

func TestScoreCompliance_General(t *testing.T) {
	score, _ := ScoreCompliance("A normal finding description that's reasonable", FmtAny)
	if score != 1.0 {
		t.Fatalf("expected 1.0, got %f", score)
	}
}

func TestScoreCompliance_GeneralShort(t *testing.T) {
	score, issues := ScoreCompliance("short", FmtAny)
	if score >= 1.0 {
		t.Fatal("expected imperfect for very short")
	}
	if len(issues) == 0 {
		t.Fatal("expected issues")
	}
}

func TestAnalyze_Empty(t *testing.T) {
	r := Analyze(nil)
	if r.TotalChecked != 0 || r.MeanScore != 0 {
		t.Fatal("expected zeros")
	}
}

func TestAnalyze_AllPerfect(t *testing.T) {
	ff := []model.Finding{
		finding("m1", "f1", "A normal finding description with enough text"),
		finding("m2", "f2", "Another normal finding description for testing"),
	}
	r := Analyze(ff)
	if r.MeanScore != 1.0 {
		t.Fatalf("expected 1.0 mean, got %f", r.MeanScore)
	}
	if len(r.Violations) != 0 {
		t.Fatal("expected no violations")
	}
}

func TestAnalyze_WithViolations(t *testing.T) {
	ff := []model.Finding{
		finding("m1", "f1", `return JSON output: {"key": broken}`),
		finding("m2", "f2", "Normal finding text is long enough here"),
	}
	r := Analyze(ff)
	if len(r.Violations) == 0 {
		t.Fatal("expected violations")
	}
}

func TestAnalyze_ByModel(t *testing.T) {
	ff := []model.Finding{
		finding("m1", "f1", ""),
		finding("m1", "f2", "A normal finding description sufficient length"),
		finding("m2", "f3", "A normal finding description sufficient length"),
	}
	r := Analyze(ff)
	if len(r.ByModel) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.ByModel))
	}
}

func TestAnalyze_WorstPrompts(t *testing.T) {
	ff := []model.Finding{
		finding("m1", "f1", ""),
		finding("m1", "f2", "Normal finding sufficient length text"),
		finding("m1", "f3", "Normal finding sufficient length text"),
	}
	r := Analyze(ff)
	if len(r.WorstPrompts) == 0 {
		t.Fatal("expected worst prompts with empty finding")
	}
}

func TestFormatText(t *testing.T) {
	ff := []model.Finding{
		finding("m1", "f1", `return JSON output: {"broken`),
	}
	r := Analyze(ff)
	text := FormatText(r)
	if !strings.Contains(text, "Format Compliance") {
		t.Fatal("expected header")
	}
	if !strings.Contains(text, "m1") {
		t.Fatal("expected model name")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestScoreCompliance_JSONWithKeys(t *testing.T) {
	score, _ := ScoreCompliance(`{"name": "test", invalid}`, FmtJSON)
	if score <= 0.4 {
		t.Fatalf("expected bonus for JSON-like keys, got %f", score)
	}
}

func TestAnalyze_ByFormat(t *testing.T) {
	ff := []model.Finding{
		finding("m1", "f1", `return JSON: {"key": "val"}`),
		finding("m1", "f2", "use markdown\n# heading\n- list"),
	}
	r := Analyze(ff)
	if len(r.ByFormat) < 2 {
		t.Fatalf("expected at least 2 formats, got %d", len(r.ByFormat))
	}
}
