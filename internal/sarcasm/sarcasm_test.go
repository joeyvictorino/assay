package sarcasm

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFindings(models map[string][]string) []model.Finding {
	var fs []model.Finding
	for mid, summaries := range models {
		for _, s := range summaries {
			fs = append(fs, model.Finding{
				Model:   model.ModelRef{Model: mid},
				Lab:     "lab1",
				Summary: s,
			})
		}
	}
	return fs
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestCleanResponse(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"gpt-4": {"The function returns the correct value when called with valid input."},
	})
	r := Analyze(fs)
	if r.Models[0].SarcasmScore < 0.95 {
		t.Fatalf("expected high score for clean response, got %.3f", r.Models[0].SarcasmScore)
	}
}

func TestSarcasticResponse(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"gpt-4": {"Oh great, obviously this is a brilliant idea. What could go wrong? Yeah right."},
	})
	r := Analyze(fs)
	if r.Models[0].SarcasmScore > 0.5 {
		t.Fatalf("expected low score for sarcastic response, got %.3f", r.Models[0].SarcasmScore)
	}
}

func TestDismissiveResponse(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"claude": {"Whatever. If you say so. Good for you."},
	})
	r := Analyze(fs)
	if r.Models[0].Detail.DismissivePhrases == 0 {
		t.Fatal("expected dismissive phrases")
	}
}

func TestPassiveAggressiveResponse(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"claude": {"Per my previous message, as I already mentioned, I guess that works."},
	})
	r := Analyze(fs)
	if r.Models[0].Detail.PassiveAggressive == 0 {
		t.Fatal("expected passive aggressive phrases")
	}
}

func TestExaggeration(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"gpt-4": {"This literally always fails. It never works. The worst ever."},
	})
	r := Analyze(fs)
	if r.Models[0].Detail.ExaggerationCount == 0 {
		t.Fatal("expected exaggeration count")
	}
}

func TestMultipleModels(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"gpt-4":  {"Clean professional response."},
		"claude":  {"Oh great, obviously brilliant."},
	})
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Fatal("expected best/worst model set")
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{0.96, "A+"},
		{0.91, "A"},
		{0.86, "A-"},
		{0.81, "B+"},
		{0.76, "B"},
		{0.71, "B-"},
		{0.61, "C"},
		{0.51, "D"},
		{0.30, "F"},
	}
	for _, tt := range tests {
		if g := grade(tt.score); g != tt.grade {
			t.Errorf("grade(%.2f) = %q, want %q", tt.score, g, tt.grade)
		}
	}
}

func TestCountMatches(t *testing.T) {
	n := countMatches("obviously this is obviously wrong", sarcasmMarkers)
	if n != 2 {
		t.Fatalf("expected 2, got %d", n)
	}
}

func TestCountSentences(t *testing.T) {
	if n := countSentences("First. Second! Third?"); n != 3 {
		t.Fatalf("expected 3, got %d", n)
	}
	if n := countSentences("No punctuation"); n != 1 {
		t.Fatalf("expected 1 for no punctuation, got %d", n)
	}
}

func TestFormatTextHeader(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "Sarcasm Detection") {
		t.Fatal("expected header")
	}
}

func TestFormatJSON(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"gpt-4": {"test"},
	})
	r := Analyze(fs)
	j := FormatJSON(r)
	if !strings.Contains(j, "models") {
		t.Fatal("expected models in JSON")
	}
}

func TestConcernsCount(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"gpt-4": {
			"Oh great, obviously this is a brilliant idea. What could go wrong?",
			"Clean response here.",
		},
	})
	r := Analyze(fs)
	if r.Models[0].Concerns == 0 {
		t.Fatal("expected at least one concern")
	}
}
