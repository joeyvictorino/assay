package hedging

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

func TestDirectResponse(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"gpt-4": {"The function returns the correct value. This implementation is secure."},
	})
	r := Analyze(fs)
	if r.Models[0].HedgingScore < 0.90 {
		t.Fatalf("expected high score for direct response, got %.3f", r.Models[0].HedgingScore)
	}
}

func TestHeavyHedging(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"claude": {"Maybe this could be somewhat of a problem. Perhaps it might possibly cause issues. I think it seems likely to a degree."},
	})
	r := Analyze(fs)
	if r.Models[0].HedgingScore > 0.70 {
		t.Fatalf("expected lower score for heavy hedging, got %.3f", r.Models[0].HedgingScore)
	}
}

func TestUncertaintyDetection(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"gpt-4": {"Maybe this is correct. Perhaps not. I think it could be right. Probably works."},
	})
	r := Analyze(fs)
	if r.Models[0].Detail.UncertaintyWords == 0 {
		t.Fatal("expected uncertainty words detected")
	}
}

func TestQualifierDetection(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"claude": {"This is somewhat correct. It is relatively simple. Fairly straightforward and rather basic."},
	})
	r := Analyze(fs)
	if r.Models[0].Detail.QualifierWords == 0 {
		t.Fatal("expected qualifier words detected")
	}
}

func TestDisclaimerDetection(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"gpt-4": {"I'm not an expert but I think this works. Please verify with a professional. I could be wrong."},
	})
	r := Analyze(fs)
	if r.Models[0].Detail.DisclaimerCount == 0 {
		t.Fatal("expected disclaimers detected")
	}
}

func TestCaveatDetection(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"claude": {"This works well. However, it's worth noting that depending on the config, one caveat applies."},
	})
	r := Analyze(fs)
	if r.Models[0].Detail.CaveatsCount == 0 {
		t.Fatal("expected caveats detected")
	}
}

func TestMultipleModels(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"gpt-4":  {"Direct clear answer."},
		"claude":  {"Maybe perhaps I think it might possibly work to some extent."},
	})
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
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

func TestCountPhrases(t *testing.T) {
	n := countPhrases("maybe this is maybe correct", uncertaintyWords)
	if n < 2 {
		t.Fatalf("expected at least 2, got %d", n)
	}
}

func TestCountWords(t *testing.T) {
	if n := countWords("one two three"); n != 3 {
		t.Fatalf("expected 3, got %d", n)
	}
}

func TestFormatTextHeader(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "Hedging Language") {
		t.Fatal("expected header")
	}
}

func TestFormatJSON(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"gpt-4": {"test response"},
	})
	r := Analyze(fs)
	j := FormatJSON(r)
	if !strings.Contains(j, "models") {
		t.Fatal("expected models in JSON")
	}
}

func TestHighHedgeCount(t *testing.T) {
	fs := mkFindings(map[string][]string{
		"gpt-4": {
			"Maybe perhaps I think possibly it might sort of kind of work to some extent.",
			"Direct answer here.",
		},
	})
	r := Analyze(fs)
	if r.Models[0].HighHedge == 0 {
		t.Fatal("expected at least one high hedge sample")
	}
}
