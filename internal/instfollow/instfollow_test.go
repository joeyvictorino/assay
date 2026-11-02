package instfollow

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(id, modelName, summary, promptHash string, state model.FindingState) model.Finding {
	return model.Finding{
		ID:         id,
		Model:      model.ModelRef{Model: modelName},
		Summary:    summary,
		PromptHash: promptHash,
		State:      state,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.TotalResponses != 0 {
		t.Fatalf("expected 0 responses, got %d", r.TotalResponses)
	}
}

func TestAnalyzeDeclinedSkipped(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "gpt-4", "refused", "p1", model.StateDeclined),
	}
	r := Analyze(ff)
	if r.TotalResponses != 0 {
		t.Fatalf("declined findings should be skipped, got %d", r.TotalResponses)
	}
}

func TestAnalyzePerfectResponse(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "claude-3", "This is a well-formed response with detailed analysis.", "p1", model.StateValidated),
	}
	r := Analyze(ff)
	if r.TotalResponses != 1 {
		t.Fatalf("expected 1 response, got %d", r.TotalResponses)
	}
	if r.OverallMean < 0.9 {
		t.Fatalf("expected high score for clean response, got %.4f", r.OverallMean)
	}
}

func TestAnalyzeEmptyResponse(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "claude-3", "", "p1", model.StateValidated),
	}
	r := Analyze(ff)
	if r.TotalResponses != 1 {
		t.Fatalf("expected 1 response, got %d", r.TotalResponses)
	}
	if r.OverallMean != 0.0 {
		t.Fatalf("expected 0 for empty response, got %.4f", r.OverallMean)
	}
	if len(r.Scores[0].Violations) == 0 {
		t.Fatal("expected violation for empty response")
	}
	if r.Scores[0].Violations[0].Type != ViolationEmpty {
		t.Fatalf("expected empty violation, got %s", r.Scores[0].Violations[0].Type)
	}
}

func TestAnalyzeShortResponse(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "gpt-4", "OK", "p1", model.StateValidated),
	}
	r := Analyze(ff)
	if r.Scores[0].Score >= 1.0 {
		t.Fatalf("short response should be penalized, got %.4f", r.Scores[0].Score)
	}
	found := false
	for _, v := range r.Scores[0].Violations {
		if v.Type == ViolationIncomplete {
			found = true
		}
	}
	if !found {
		t.Fatal("expected incomplete violation for short response")
	}
}

func TestAnalyzeOffTopicContent(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "gpt-4", "As an AI language model, I cannot provide investment advice but here is some general information about stocks.", "p1", model.StateValidated),
	}
	r := Analyze(ff)
	found := false
	for _, v := range r.Scores[0].Violations {
		if v.Type == ViolationOffTopic {
			found = true
		}
	}
	if !found {
		t.Fatal("expected off-topic violation")
	}
}

func TestAnalyzeExtraContent(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "claude-3", "The answer is 42. I hope this helps! Let me know if you have more questions.", "p1", model.StateValidated),
	}
	r := Analyze(ff)
	found := false
	for _, v := range r.Scores[0].Violations {
		if v.Type == ViolationExtraContent {
			found = true
		}
	}
	if !found {
		t.Fatal("expected extra content violation")
	}
}

func TestAnalyzeJSONFormatCompliance(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "gpt-4", `{"key": "value"}`, "json-prompt", model.StateValidated),
	}
	r := Analyze(ff)
	if r.Scores[0].Score < 0.9 {
		t.Fatalf("valid JSON should score high, got %.4f", r.Scores[0].Score)
	}
}

func TestAnalyzeJSONFormatNonCompliance(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "gpt-4", "This is not JSON at all", "json-prompt", model.StateValidated),
	}
	r := Analyze(ff)
	found := false
	for _, v := range r.Scores[0].Violations {
		if v.Type == ViolationFormatMismatch {
			found = true
		}
	}
	if !found {
		t.Fatal("expected format mismatch for non-JSON response to JSON prompt")
	}
}

func TestAnalyzeListFormatCompliance(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "claude-3", "- item one\n- item two\n- item three", "list-prompt", model.StateValidated),
	}
	r := Analyze(ff)
	if r.Scores[0].Score < 0.9 {
		t.Fatalf("valid list should score high, got %.4f", r.Scores[0].Score)
	}
}

func TestAnalyzeMarkdownCompliance(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "claude-3", "# Header\n\nSome **bold** text\n\n```code```", "markdown-response", model.StateValidated),
	}
	r := Analyze(ff)
	if r.Scores[0].Score < 0.9 {
		t.Fatalf("valid markdown should score high, got %.4f", r.Scores[0].Score)
	}
}

func TestMultipleModels(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "gpt-4", "Good response with enough content here.", "p1", model.StateValidated),
		finding("f2", "claude-3", "", "p2", model.StateValidated),
		finding("f3", "gpt-4", "Another good response from gpt-4 model.", "p3", model.StateValidated),
	}
	r := Analyze(ff)
	if r.TotalResponses != 3 {
		t.Fatalf("expected 3, got %d", r.TotalResponses)
	}
	if len(r.ByModel) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.ByModel))
	}
}

func TestFailurePatterns(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "m1", "", "p1", model.StateValidated),
		finding("f2", "m1", "", "p2", model.StateValidated),
		finding("f3", "m1", "OK", "p3", model.StateValidated),
	}
	r := Analyze(ff)
	if len(r.FailurePatterns) == 0 {
		t.Fatal("expected failure patterns")
	}
	var emptyCount int
	for _, fp := range r.FailurePatterns {
		if fp.Type == ViolationEmpty {
			emptyCount = fp.Count
		}
	}
	if emptyCount != 2 {
		t.Fatalf("expected 2 empty violations, got %d", emptyCount)
	}
}

func TestUnknownModel(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "", "A normal response with enough text.", "p1", model.StateValidated),
	}
	r := Analyze(ff)
	if r.ByModel[0].Model != "unknown" {
		t.Fatalf("expected 'unknown' model, got %q", r.ByModel[0].Model)
	}
}

func TestFormatText(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "gpt-4", "Some valid response text.", "p1", model.StateValidated),
	}
	r := Analyze(ff)
	text := FormatText(r)
	if !strings.Contains(text, "Instruction Following") {
		t.Fatal("missing header in text output")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Fatal("missing model in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	ff := []model.Finding{
		finding("f1", "claude-3", "Response text.", "p1", model.StateValidated),
	}
	r := Analyze(ff)
	out := FormatJSON(r)
	var data map[string]any
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if _, ok := data["total_responses"]; !ok {
		t.Fatal("missing total_responses in JSON")
	}
}

func TestTableFormatDetection(t *testing.T) {
	score := checkFormatCompliance("| col1 | col2 |\n| --- | --- |\n| a | b |", "table")
	if score < 0.9 {
		t.Fatalf("valid table should score high, got %.4f", score)
	}
}

func TestCSVFormatDetection(t *testing.T) {
	score := checkFormatCompliance("name,age,city\nAlice,30,NYC\nBob,25,LA", "csv")
	if score < 0.9 {
		t.Fatalf("valid CSV should score high, got %.4f", score)
	}
}

func TestCodeFormatDetection(t *testing.T) {
	score := checkFormatCompliance("```python\nprint('hello')\n```", "code")
	if score < 0.9 {
		t.Fatalf("valid code block should score high, got %.4f", score)
	}
}

func TestXMLFormatDetection(t *testing.T) {
	score := checkFormatCompliance("<root><item>test</item></root>", "xml")
	if score < 0.9 {
		t.Fatalf("valid XML should score high, got %.4f", score)
	}
}

func TestYAMLFormatDetection(t *testing.T) {
	score := checkFormatCompliance("key: value\nlist:\n  - item1\n  - item2", "yaml")
	if score < 0.9 {
		t.Fatalf("valid YAML should score high, got %.4f", score)
	}
}
