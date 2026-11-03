package promptcomplexity

import (
	"strings"
	"testing"
)

func TestEmptyPrompt(t *testing.T) {
	r := Analyze("")
	if r.Score != 100 {
		t.Fatalf("expected score 100, got %d", r.Score)
	}
	if r.Grade != "A" {
		t.Fatalf("expected grade A, got %s", r.Grade)
	}
}

func TestSimplePrompt(t *testing.T) {
	r := Analyze("What is the capital of France?")
	if r.Grade != "A" {
		t.Fatalf("expected grade A for simple prompt, got %s", r.Grade)
	}
	if r.Metrics.QuestionCount != 1 {
		t.Fatalf("expected 1 question, got %d", r.Metrics.QuestionCount)
	}
}

func TestComplexPrompt(t *testing.T) {
	prompt := `If the user provides a file, you must always validate it first.
When the file is invalid, never proceed. Unless the user says otherwise,
you shall only process JSON files. If the format is not JSON, you must
reject it. Step 1: validate. Step 2: parse. Step 3: transform.
Step 4: output. Don't skip steps. Can't you see the above instructions?
Do not deviate from the previous requirements. When in doubt, always
check the earlier constraints. If (nested (deeply)) then fail.`
	r := Analyze(prompt)
	if r.Score <= 40 {
		t.Fatalf("expected high complexity score, got %d", r.Score)
	}
	if r.Metrics.ConditionalCount < 3 {
		t.Fatalf("expected multiple conditionals, got %d", r.Metrics.ConditionalCount)
	}
	if r.Metrics.ConstraintCount < 3 {
		t.Fatalf("expected multiple constraints, got %d", r.Metrics.ConstraintCount)
	}
}

func TestScoreIncreasesWithComplexity(t *testing.T) {
	simple := Analyze("Hello world.")
	complex := Analyze("If x, then you must always do y. When z, never do w. Unless a, shall b. Step 1: c. Step 2: d. Can't skip? Don't forget the above.")
	if complex.Score <= simple.Score {
		t.Fatal("expected higher score for more complex prompt")
	}
}

func TestFormatTextContainsHeader(t *testing.T) {
	r := Analyze("test prompt")
	out := FormatText(r)
	if !strings.Contains(out, "Prompt Complexity Analysis") {
		t.Fatal("expected header")
	}
}

func TestGradeBoundaries(t *testing.T) {
	tests := []struct {
		score int
		grade string
	}{
		{0, "A"}, {19, "A"}, {20, "B"}, {39, "B"},
		{40, "C"}, {59, "C"}, {60, "D"}, {79, "D"},
		{80, "F"}, {100, "F"},
	}
	for _, tt := range tests {
		got := scoreGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreGrade(%d) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestNestingDepth(t *testing.T) {
	r := Analyze("a (b [c {d}])")
	if r.Metrics.NestingDepth != 3 {
		t.Fatalf("expected nesting depth 3, got %d", r.Metrics.NestingDepth)
	}
}

func TestRecommendations(t *testing.T) {
	words := make([]string, 250)
	for i := range words {
		words[i] = "word"
	}
	r := Analyze(strings.Join(words, " "))
	found := false
	for _, rec := range r.Recommendations {
		if strings.Contains(rec, "breaking") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected recommendation about breaking long prompt")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze("test")
	out := FormatJSON(r)
	if !strings.Contains(out, "token_count") {
		t.Fatal("expected JSON field token_count")
	}
}
