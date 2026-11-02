package contextwin

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(summary string) model.Finding {
	return model.Finding{ID: "f1", Summary: summary, Lab: "checker-a", Model: model.ModelRef{Model: "claude-3-opus"}}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if r.Summary.TotalFindings != 0 {
		t.Fatalf("expected 0, got %d", r.Summary.TotalFindings)
	}
}

func TestSingleFinding(t *testing.T) {
	r := Analyze([]model.Finding{finding("the application has a critical vulnerability in the login handler")})
	if r.Summary.TotalFindings != 1 {
		t.Fatalf("expected 1, got %d", r.Summary.TotalFindings)
	}
	if len(r.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(r.Runs))
	}
	if r.Runs[0].ContextWindow != 200000 {
		t.Fatalf("expected 200000 context window for claude-3-opus, got %d", r.Runs[0].ContextWindow)
	}
}

func TestUnknownModel(t *testing.T) {
	f := model.Finding{ID: "f1", Summary: "test finding", Lab: "check", Model: model.ModelRef{Model: "unknown-model"}}
	r := Analyze([]model.Finding{f})
	if len(r.Runs) != 1 {
		t.Fatal("expected 1 run")
	}
	if r.Runs[0].ContextWindow != defaultContextWindow {
		t.Fatalf("expected default context window %d, got %d", defaultContextWindow, r.Runs[0].ContextWindow)
	}
}

func TestRepeatDetection(t *testing.T) {
	repeated := "the quick brown fox jumps over the lazy dog again"
	ff := []model.Finding{
		{ID: "f1", Summary: repeated, Lab: "check", Model: model.ModelRef{Model: "gpt-4"}},
		{ID: "f2", Summary: repeated, Lab: "check", Model: model.ModelRef{Model: "gpt-4"}},
		{ID: "f3", Summary: repeated, Lab: "check", Model: model.ModelRef{Model: "gpt-4"}},
	}
	r := Analyze(ff)
	if len(r.Runs) != 1 {
		t.Fatalf("expected 1 aggregated run, got %d", len(r.Runs))
	}
	if r.Runs[0].RepeatRatio <= 0 {
		t.Fatalf("expected non-zero repeat ratio for repeated content, got %.1f", r.Runs[0].RepeatRatio)
	}
}

func TestMultipleModels(t *testing.T) {
	ff := []model.Finding{
		{ID: "f1", Summary: "test one", Lab: "check", Model: model.ModelRef{Model: "gpt-4"}},
		{ID: "f2", Summary: "test two", Lab: "check", Model: model.ModelRef{Model: "claude-3-opus"}},
	}
	r := Analyze(ff)
	if len(r.Runs) != 2 {
		t.Fatalf("expected 2 runs for 2 models, got %d", len(r.Runs))
	}
}

func TestMultipleCheckers(t *testing.T) {
	ff := []model.Finding{
		{ID: "f1", Summary: "test one", Lab: "checker-a", Model: model.ModelRef{Model: "gpt-4"}},
		{ID: "f2", Summary: "test two", Lab: "checker-b", Model: model.ModelRef{Model: "gpt-4"}},
	}
	r := Analyze(ff)
	if len(r.Runs) != 2 {
		t.Fatalf("expected 2 runs for 2 checkers, got %d", len(r.Runs))
	}
}

func TestLowUtilSuggestion(t *testing.T) {
	f := model.Finding{ID: "f1", Summary: "short", Lab: "check", Model: model.ModelRef{Model: "gemini-1.5-pro"}}
	r := Analyze([]model.Finding{f})
	found := false
	for _, s := range r.Suggestions {
		if s.Type == "batching" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected batching suggestion for low utilization on large context window")
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze([]model.Finding{finding("test content here for analysis")})
	text := FormatText(r)
	if !strings.Contains(text, "Context Window") {
		t.Fatal("expected header in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("test content here")})
	j := FormatJSON(r)
	if j == "" || j[0] != '{' {
		t.Fatal("expected valid JSON")
	}
}

func TestEvidenceIncluded(t *testing.T) {
	f := model.Finding{
		ID: "f1", Summary: "summary", Lab: "check",
		Model:    model.ModelRef{Model: "claude-3-opus"},
		Evidence: []model.Evidence{{Kind: "tool-output", Marker: "detailed evidence content with many words for token estimation"}},
	}
	r := Analyze([]model.Finding{f})
	if r.Runs[0].EstPromptTokens <= 2 {
		t.Fatal("expected evidence words to contribute to token estimate")
	}
}
