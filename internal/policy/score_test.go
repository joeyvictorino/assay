package policy

import (
	"strings"
	"testing"
)

func TestScoreStrictTemplate(t *testing.T) {
	yaml, _ := GenerateTemplate("strict")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	r := Score(eng)
	if r.OverallScore < 50 || r.OverallScore > 100 {
		t.Errorf("score = %.1f, want 50-100", r.OverallScore)
	}
	if r.Grade == "" {
		t.Error("grade should not be empty")
	}
	if len(r.AgentScores) == 0 {
		t.Error("should have agent scores")
	}
	if len(r.CategoryScores) == 0 {
		t.Error("should have category scores")
	}
	if r.Summary == "" {
		t.Error("summary should not be empty")
	}
}

func TestScoreMinimalTemplate(t *testing.T) {
	yaml, _ := GenerateTemplate("minimal")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	r := Score(eng)
	if r.OverallScore < 0 || r.OverallScore > 100 {
		t.Errorf("score = %.1f, out of range", r.OverallScore)
	}
	if len(r.Deductions) == 0 {
		t.Error("minimal template should have deductions")
	}
}

func TestScoreGrading(t *testing.T) {
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
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %q, want %q", tt.score, got, tt.grade)
		}
	}
}

func TestFormatScoreText(t *testing.T) {
	yaml, _ := GenerateTemplate("strict")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	r := Score(eng)
	text := FormatScoreText(r)
	if text == "" {
		t.Error("formatted text should not be empty")
	}
	if !strings.Contains(text, "Security Posture Score") {
		t.Error("should contain header")
	}
	if !strings.Contains(text, "Grade:") {
		t.Error("should contain grade")
	}
}
