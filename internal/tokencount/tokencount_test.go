package tokencount

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(mdl, checker, prompt, summary string) model.Finding {
	f := model.Finding{
		ID:         "f-" + mdl + "-" + prompt,
		RunID:      "run-1",
		Class:      model.ClassSQLi,
		Severity:   model.SevHigh,
		State:      model.StateValidated,
		Summary:    summary,
		Model:      model.ModelRef{Model: mdl, Provider: "test", Agent: "agent-1"},
		PromptHash: prompt,
		FirstSeen:  time.Now(),
		LastSeen:   time.Now(),
	}
	if checker != "" {
		f.Validation = &model.ValidationResult{
			Checker: checker,
			State:   model.StateValidated,
		}
	}
	return f
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.TotalTokens != 0 || r.TotalFindings != 0 {
		t.Fatal("expected zero report")
	}
}

func TestAnalyzeSingleFinding(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "sqli-check", "p1", "SQL injection in login endpoint"),
	}
	r := Analyze(findings)
	if r.TotalFindings != 1 {
		t.Fatalf("expected 1 finding, got %d", r.TotalFindings)
	}
	if r.TotalTokens <= 0 {
		t.Fatal("expected positive token count")
	}
	if len(r.ByModel) != 1 || r.ByModel[0].Model != "gpt-4" {
		t.Fatal("expected one model entry")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "sqli-check", "p1", "SQL injection found"),
		mkFinding("claude-3", "sqli-check", "p1", "SQL injection detected in parameter"),
		mkFinding("gpt-4", "xss-check", "p2", "XSS reflected in search"),
	}
	r := Analyze(findings)
	if len(r.ByModel) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.ByModel))
	}
	if r.TotalFindings != 3 {
		t.Fatalf("expected 3 findings, got %d", r.TotalFindings)
	}
}

func TestAnalyzeMultipleCheckers(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "sqli-check", "p1", "SQL injection"),
		mkFinding("gpt-4", "xss-check", "p2", "XSS reflected"),
		mkFinding("gpt-4", "auth-check", "p3", "Missing authentication"),
	}
	r := Analyze(findings)
	if len(r.ByChecker) != 3 {
		t.Fatalf("expected 3 checkers, got %d", len(r.ByChecker))
	}
}

func TestAnalyzePromptBreakdown(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "sqli-check", "p1", "finding one"),
		mkFinding("gpt-4", "sqli-check", "p1", "finding two is longer text"),
		mkFinding("gpt-4", "xss-check", "p2", "xss finding"),
	}
	r := Analyze(findings)
	if len(r.ByPrompt) < 2 {
		t.Fatalf("expected at least 2 prompts, got %d", len(r.ByPrompt))
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"", 0},
		{"hello", 2},                                  // 1 word * 1.3 = 1.3 -> ceil = 2
		{"hello world", 3},                            // 2 words * 1.3 = 2.6 -> ceil = 3
		{"one two three four five", 7},                // 5 words * 1.3 = 6.5 -> ceil = 7
	}
	for _, tc := range tests {
		got := estimateTokens(tc.text)
		if got != tc.want {
			t.Errorf("estimateTokens(%q) = %d, want %d", tc.text, got, tc.want)
		}
	}
}

func TestFindingsPer1K(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "check", "p1", "a b c d e f g h i j"),
		mkFinding("gpt-4", "check", "p2", "a b c d e f g h i j"),
	}
	r := Analyze(findings)
	if r.OverallFPer1K <= 0 {
		t.Fatal("expected positive findings per 1K tokens")
	}
	for _, ms := range r.ByModel {
		if ms.FindingsPer1K <= 0 {
			t.Fatalf("model %s: expected positive findings per 1K", ms.Model)
		}
	}
}

func TestTokensPerFinding(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "check", "p1", "hello world again"),
		mkFinding("gpt-4", "check", "p1", "another finding text here"),
	}
	r := Analyze(findings)
	found := false
	for _, ps := range r.ByPrompt {
		if ps.FindingCount > 0 {
			if ps.TokensPerFind <= 0 {
				t.Fatal("expected positive tokens per finding")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("no prompt stats with findings")
	}
}

func TestAvgTokensPerPrompt(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "check", "p1", "first finding"),
		mkFinding("gpt-4", "check2", "p2", "second finding with more words"),
	}
	r := Analyze(findings)
	if len(r.ByModel) != 1 {
		t.Fatal("expected 1 model")
	}
	if r.ByModel[0].AvgTokensPerPrompt <= 0 {
		t.Fatal("expected positive avg tokens per prompt")
	}
}

func TestUnknownChecker(t *testing.T) {
	f := model.Finding{
		ID:       "f1",
		RunID:    "run-1",
		Class:    model.ClassXSSReflected,
		Severity: model.SevMedium,
		State:    model.StateTheorized,
		Summary:  "XSS in search",
		Model:    model.ModelRef{Model: "gpt-4"},
	}
	r := Analyze([]model.Finding{f})
	if len(r.ByChecker) != 1 || r.ByChecker[0].Checker != "unknown" {
		t.Fatal("expected unknown checker")
	}
}

func TestUnknownPromptHash(t *testing.T) {
	f := model.Finding{
		ID:       "f1",
		RunID:    "run-1",
		Class:    model.ClassSQLi,
		Severity: model.SevHigh,
		State:    model.StateValidated,
		Summary:  "sql injection",
		Model:    model.ModelRef{Model: "gpt-4"},
	}
	r := Analyze([]model.Finding{f})
	for _, ps := range r.ByPrompt {
		if ps.PromptHash != "unknown" {
			t.Fatal("expected unknown prompt hash")
		}
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "sqli-check", "p1", "SQL injection in login"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Token Count Analysis") {
		t.Fatal("missing header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Fatal("missing model name")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "sqli-check", "p1", "SQL injection"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.TotalFindings != 1 {
		t.Fatalf("expected 1 finding in JSON, got %d", parsed.TotalFindings)
	}
}

func TestSortedByTotalTokens(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "check", "p1", "short"),
		mkFinding("claude-3", "check", "p2", "this is a much longer summary with many more words to count"),
	}
	r := Analyze(findings)
	if len(r.ByModel) < 2 {
		t.Skip("need 2 models")
	}
	if r.ByModel[0].TotalTokens < r.ByModel[1].TotalTokens {
		t.Fatal("models should be sorted descending by total tokens")
	}
}

func TestCheckerPromptCount(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "sqli-check", "p1", "finding one"),
		mkFinding("gpt-4", "sqli-check", "p2", "finding two"),
		mkFinding("gpt-4", "xss-check", "p3", "finding three"),
	}
	r := Analyze(findings)
	for _, cs := range r.ByChecker {
		if cs.Checker == "sqli-check" && cs.PromptCount != 2 {
			t.Fatalf("sqli-check: expected 2 prompts, got %d", cs.PromptCount)
		}
		if cs.Checker == "xss-check" && cs.PromptCount != 1 {
			t.Fatalf("xss-check: expected 1 prompt, got %d", cs.PromptCount)
		}
	}
}

func TestTotalPromptCount(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "check", "p1", "f1"),
		mkFinding("gpt-4", "check", "p1", "f2"),
		mkFinding("claude", "check2", "p2", "f3"),
	}
	r := Analyze(findings)
	if r.TotalPrompts < 2 {
		t.Fatalf("expected at least 2 prompts, got %d", r.TotalPrompts)
	}
}
