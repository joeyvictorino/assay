package chainquality

import (
	"encoding/json"
	"testing"
)

func TestEmptyText(t *testing.T) {
	r := Analyze("")
	if r.OverallScore < 0 || r.OverallScore > 100 {
		t.Errorf("score out of bounds: %.2f", r.OverallScore)
	}
	if r.Grade == "" {
		t.Error("expected non-empty grade")
	}
}

func TestWellStructuredReasoning(t *testing.T) {
	text := `1. First, we need to identify the given values. Based on the context, we have x=5.
2. Next, because x=5, we can calculate y = x*2 = 10. Therefore the intermediate result is 10.
3. Finally, since y=10, the answer follows. As stated in the problem, the result is thus 10.
In conclusion, the answer is 10 because each step logically follows from the previous one.`
	r := Analyze(text)
	if r.OverallScore < 60 {
		t.Errorf("expected high score for well-structured reasoning, got %.2f", r.OverallScore)
	}
	if r.StepCount < 3 {
		t.Errorf("expected at least 3 steps, got %d", r.StepCount)
	}
}

func TestBareAnswer(t *testing.T) {
	r := Analyze("42")
	if r.OverallScore > 70 {
		t.Errorf("expected lower score for bare answer, got %.2f", r.OverallScore)
	}
}

func TestCompletenessWithConclusion(t *testing.T) {
	text := "We start with the premise. Then we analyze. Therefore the answer is clear."
	r := Analyze(text)
	var completeness float64
	for _, d := range r.Dimensions {
		if d.Name == "reasoning_completeness" {
			completeness = d.Score
		}
	}
	if completeness < 50 {
		t.Errorf("expected decent completeness with conclusion, got %.2f", completeness)
	}
}

func TestLogicalConsistencyWithTransitions(t *testing.T) {
	text := "First, we consider A. Because A is true, therefore B follows. Since B, consequently C must hold. Thus the result is clear. Moreover, this is confirmed by D. Furthermore, E supports this."
	r := Analyze(text)
	var consistency float64
	for _, d := range r.Dimensions {
		if d.Name == "logical_consistency" {
			consistency = d.Score
		}
	}
	if consistency < 60 {
		t.Errorf("expected high consistency with transitions, got %.2f", consistency)
	}
}

func TestEvidenceGrounding(t *testing.T) {
	text := "According to the context, the value is 5. Based on the provided data, we compute the result. As stated in the document, the answer confirms our calculation."
	r := Analyze(text)
	var grounding float64
	for _, d := range r.Dimensions {
		if d.Name == "evidence_grounding" {
			grounding = d.Score
		}
	}
	if grounding < 50 {
		t.Errorf("expected decent grounding score, got %.2f", grounding)
	}
}

func TestWeakestLinkIdentified(t *testing.T) {
	r := Analyze("The answer is 42.")
	if r.WeakestLink == "" {
		t.Error("expected weakest link to be identified")
	}
}

func TestGradeMapping(t *testing.T) {
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
		g := gradeFromScore(tc.score)
		if g != tc.grade {
			t.Errorf("score %.0f: expected grade %s, got %s", tc.score, tc.grade, g)
		}
	}
}

func TestBatchAnalysis(t *testing.T) {
	texts := []string{
		"First, because of X. Therefore Y. In conclusion Z.",
		"The answer is 42.",
	}
	br := AnalyzeBatch(texts)
	if len(br.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(br.Results))
	}
	if br.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestBatchEmpty(t *testing.T) {
	br := AnalyzeBatch(nil)
	if len(br.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(br.Results))
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze("First step. Therefore second step. In conclusion, done.")
	text := FormatText(r)
	if text == "" {
		t.Error("expected non-empty text output")
	}
	if !containsStr(text, "Chain-of-Thought") {
		t.Error("expected header in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze("A test reasoning chain.")
	out := FormatJSON(r)
	var parsed ChainQualityReport
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.Grade == "" {
		t.Error("expected grade in JSON")
	}
}

func TestScoreBounds(t *testing.T) {
	texts := []string{
		"",
		"Short.",
		"A very long reasoning chain with many steps and lots of evidence markers. According to the data, first we note X. Based on the provided context, because of Y, therefore Z follows. In conclusion the answer is clear. Furthermore, the text states additional support. As mentioned in the document, this confirms our finding. Given that all premises hold, consequently the result is validated. Moreover, step by step analysis shows correctness. Finally, to summarize, the complete chain demonstrates the answer.",
	}
	for _, text := range texts {
		r := Analyze(text)
		if r.OverallScore < 0 || r.OverallScore > 100 {
			t.Errorf("score out of bounds: %.2f for text len %d", r.OverallScore, len(text))
		}
		for _, d := range r.Dimensions {
			if d.Score < 0 || d.Score > 100 {
				t.Errorf("dimension %s score out of bounds: %.2f", d.Name, d.Score)
			}
		}
	}
}

func TestDimensionCount(t *testing.T) {
	r := Analyze("Any reasoning text.")
	if len(r.Dimensions) != 3 {
		t.Errorf("expected 3 dimensions, got %d", len(r.Dimensions))
	}
}

func TestCountStepsNumbered(t *testing.T) {
	text := "1. First\n2. Second\n3. Third"
	count := countSteps(text)
	if count != 3 {
		t.Errorf("expected 3 steps, got %d", count)
	}
}

func TestCountStepsBulleted(t *testing.T) {
	text := "- Step one\n- Step two\n- Step three\n- Step four"
	count := countSteps(text)
	if count != 4 {
		t.Errorf("expected 4 steps, got %d", count)
	}
}

func TestCountStepsProse(t *testing.T) {
	text := "First sentence. Second sentence. Third sentence."
	count := countSteps(text)
	if count < 1 {
		t.Error("expected at least 1 step for prose")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
