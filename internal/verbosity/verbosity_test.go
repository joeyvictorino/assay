package verbosity

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func findings(summaries ...string) []model.Finding {
	var ff []model.Finding
	for _, s := range summaries {
		ff = append(ff, model.Finding{
			Summary:  s,
			Model:    model.ModelRef{Model: "test-model"},
			Severity: "medium",
		})
	}
	return ff
}

func TestClassifyExcessive(t *testing.T) {
	f := model.Finding{Summary: "It's worth noting that in conclusion the model was verbose"}
	l := classifyLevel(f)
	if l != LevelExcessive {
		t.Errorf("expected LevelExcessive, got %s", l)
	}
}

func TestClassifyTerse(t *testing.T) {
	f := model.Finding{Summary: "yes"}
	l := classifyLevel(f)
	if l != LevelTerse {
		t.Errorf("expected LevelTerse, got %s", l)
	}
}

func TestClassifyVerbose(t *testing.T) {
	words := strings.Repeat("word ", 201)
	f := model.Finding{Summary: words}
	l := classifyLevel(f)
	if l != LevelVerbose {
		t.Errorf("expected LevelVerbose, got %s", l)
	}
}

func TestClassifyConcise(t *testing.T) {
	f := model.Finding{Summary: "short answer here"}
	l := classifyLevel(f)
	if l != LevelConcise {
		t.Errorf("expected LevelConcise, got %s", l)
	}
}

func TestClassifyModerate(t *testing.T) {
	words := strings.Repeat("word ", 50)
	f := model.Finding{Summary: words}
	l := classifyLevel(f)
	if l != LevelModerate {
		t.Errorf("expected LevelModerate, got %s", l)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected no models for nil findings")
	}
}

func TestAnalyzeSingle(t *testing.T) {
	ff := findings("It's worth noting that this response is quite verbose")
	r := Analyze(ff)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].TotalFindings != 1 {
		t.Errorf("expected 1 finding, got %d", r.Models[0].TotalFindings)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	ff := []model.Finding{
		{Summary: "verbose response", Model: model.ModelRef{Model: "gpt-4"}, Severity: "medium"},
		{Summary: "concise", Model: model.ModelRef{Model: "claude-3"}, Severity: "low"},
	}
	r := Analyze(ff)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0.95, "A"},
		{0.85, "B"},
		{0.75, "C"},
		{0.65, "D"},
		{0.45, "F"},
	}
	for _, tt := range tests {
		got := grade(tt.score)
		if got != tt.want {
			t.Errorf("grade(%f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestBestWorst(t *testing.T) {
	ff := []model.Finding{
		{Summary: "it's worth noting furthermore moreover additionally", Model: model.ModelRef{Model: "verbose-model"}, Severity: "high"},
		{Summary: "ok", Model: model.ModelRef{Model: "terse-model"}, Severity: "low"},
	}
	r := Analyze(ff)
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best and worst models to be set")
	}
}

func TestFormatText(t *testing.T) {
	ff := findings("it's worth noting verbosity")
	r := Analyze(ff)
	text := FormatText(r)
	if text == "" {
		t.Fatal("empty text output")
	}
	if !strings.Contains(text, "Verbosity Analysis") {
		t.Error("missing header")
	}
}

func TestFormatJSON(t *testing.T) {
	ff := findings("it's worth noting verbosity")
	r := Analyze(ff)
	output := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestScoreRange(t *testing.T) {
	ff := findings("it's worth noting", "furthermore verbose", "in conclusion")
	r := Analyze(ff)
	for _, m := range r.Models {
		if m.OverallScore < 0 || m.OverallScore > 1 {
			t.Errorf("score %f out of [0,1] range", m.OverallScore)
		}
	}
}

func TestVerbosityIndex(t *testing.T) {
	words := strings.Repeat("word ", 201)
	ff := findings(words, "short")
	r := Analyze(ff)
	if r.Models[0].VerbosityIndex < 0 {
		t.Error("verbosity index should be >= 0")
	}
}
