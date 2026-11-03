package answercompleteness

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func findings(mods ...string) []model.Finding {
	var fs []model.Finding
	for i, m := range mods {
		fs = append(fs, model.Finding{
			ID:      m,
			Lab:     "lab-1",
			Summary: m,
			Model:   model.ModelRef{Model: "test-model"},
			Location: model.Location{
				PathTemplate: func() string {
					if i%2 == 0 {
						return "What is X? How does Y work? Why is Z important?"
					}
					return "Explain the concept"
				}(),
			},
		})
	}
	return fs
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
	if r.MeanScore != 0 {
		t.Fatalf("expected 0 mean score, got %.1f", r.MeanScore)
	}
}

func TestSingleFinding(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1", Summary: "X is a thing that does stuff",
			Model:    model.ModelRef{Model: "gpt-4"},
			Location: model.Location{PathTemplate: "What is X?"},
		},
	}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Fatalf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 1 {
		t.Fatalf("expected 1 sample, got %d", r.Models[0].SampleCount)
	}
}

func TestMultiQuestionPrompt(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary:  "X is defined as a concept in science. Y works through mechanisms A and B. Z is important because of reasons.",
			Model:    model.ModelRef{Model: "claude"},
			Location: model.Location{PathTemplate: "What is X? How does Y work? Why is Z important?"},
		},
	}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Detail.SubQuestionsTotal < 3 {
		t.Fatalf("expected at least 3 sub-questions, got %d", r.Models[0].Detail.SubQuestionsTotal)
	}
}

func TestFullCoverage(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary:  "Python is a programming language. JavaScript is used for web development.",
			Model:    model.ModelRef{Model: "claude"},
			Location: model.Location{PathTemplate: "What is Python? What is JavaScript?"},
		},
	}
	r := Analyze(fs)
	if r.Models[0].CompletenessScore <= 0 {
		t.Fatalf("expected positive score for covered response, got %.1f", r.Models[0].CompletenessScore)
	}
}

func TestPartialCoverage(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary:  "Python is a language.",
			Model:    model.ModelRef{Model: "claude"},
			Location: model.Location{PathTemplate: "What is Python? How does garbage collection work? Explain type hints."},
		},
	}
	r := Analyze(fs)
	if r.Models[0].Detail.FullAnswers > r.Models[0].SampleCount {
		t.Fatalf("full answers should not exceed sample count")
	}
}

func TestMissingCoverage(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary:  "I cannot help with that.",
			Model:    model.ModelRef{Model: "claude"},
			Location: model.Location{PathTemplate: "Explain quantum computing? Describe neural networks? How do transformers work?"},
		},
	}
	r := Analyze(fs)
	if r.Models[0].CompletenessScore > 80 {
		t.Fatalf("expected low score for non-answer, got %.1f", r.Models[0].CompletenessScore)
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary:  "Full detailed answer about Python and JavaScript",
			Model:    model.ModelRef{Model: "model-a"},
			Location: model.Location{PathTemplate: "Explain Python and JavaScript"},
		},
		{
			ID: "f2", Lab: "lab-1",
			Summary:  "Brief",
			Model:    model.ModelRef{Model: "model-b"},
			Location: model.Location{PathTemplate: "Explain Python and JavaScript"},
		},
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostComplete == "" || r.LeastComplete == "" {
		t.Fatal("expected most/least complete to be set")
	}
}

func TestSortByScore(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary:  "ok",
			Model:    model.ModelRef{Model: "low-model"},
			Location: model.Location{PathTemplate: "Explain A? Explain B? Explain C?"},
		},
		{
			ID: "f2", Lab: "lab-1",
			Summary:  "A is great, B is better, C is best explanation of each concept",
			Model:    model.ModelRef{Model: "high-model"},
			Location: model.Location{PathTemplate: "Explain A? Explain B? Explain C?"},
		},
	}
	r := Analyze(fs)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].CompletenessScore < r.Models[1].CompletenessScore {
		t.Fatal("models should be sorted by score descending")
	}
}

func TestGradeAssignment(t *testing.T) {
	cases := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {45, "F"},
	}
	for _, tc := range cases {
		got := scoreToGrade(tc.score)
		if got != tc.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tc.score, got, tc.grade)
		}
	}
}

func TestCountSubQuestions_MultipleQMarks(t *testing.T) {
	got := countSubQuestions("What is A? How does B work? Why C?")
	if got != 3 {
		t.Fatalf("expected 3 sub-questions, got %d", got)
	}
}

func TestCountSubQuestions_NumberedList(t *testing.T) {
	prompt := "Answer these:\n1. What is A\n2. What is B\n3. What is C"
	got := countSubQuestions(prompt)
	if got < 3 {
		t.Fatalf("expected at least 3, got %d", got)
	}
}

func TestCountSubQuestions_Conjunctions(t *testing.T) {
	prompt := "Explain X and also describe Y"
	got := countSubQuestions(prompt)
	if got < 2 {
		t.Fatalf("expected at least 2, got %d", got)
	}
}

func TestCountSubQuestions_SingleQuestion(t *testing.T) {
	got := countSubQuestions("What is Python?")
	if got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}

func TestExtractKeyAspects_NumberedList(t *testing.T) {
	aspects := extractKeyAspects("1. First topic\n2. Second topic")
	if len(aspects) < 2 {
		t.Fatalf("expected at least 2 aspects, got %d", len(aspects))
	}
}

func TestExtractKeyAspects_QuestionMarks(t *testing.T) {
	aspects := extractKeyAspects("What is X? How does Y work?")
	if len(aspects) < 2 {
		t.Fatalf("expected at least 2 aspects, got %d", len(aspects))
	}
}

func TestAspectCoverage_Full(t *testing.T) {
	cov := aspectCoverage("python programming language", "python is a programming language used widely")
	if cov < 50 {
		t.Fatalf("expected high coverage, got %.1f", cov)
	}
}

func TestAspectCoverage_None(t *testing.T) {
	cov := aspectCoverage("quantum computing qubits", "the weather is nice today")
	if cov > 30 {
		t.Fatalf("expected low coverage, got %.1f", cov)
	}
}

func TestFormatText_Empty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "Answer Completeness") {
		t.Fatal("expected header in text output")
	}
	if !strings.Contains(text, "No completeness data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatText_WithData(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary:  "Detailed answer about the topic with full coverage",
			Model:    model.ModelRef{Model: "claude-3"},
			Location: model.Location{PathTemplate: "Explain the topic in detail"},
		},
	}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "claude-3") {
		t.Fatal("expected model name in output")
	}
	if !strings.Contains(text, "sub_questions_covered") {
		t.Fatal("expected detail fields in output")
	}
}

func TestFormatJSON(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary:  "Answer about topic",
			Model:    model.ModelRef{Model: "test-model"},
			Location: model.Location{PathTemplate: "What is topic?"},
		},
	}
	r := Analyze(fs)
	out := FormatJSON(r)
	var data map[string]any
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Fatal("expected models key in JSON")
	}
	if _, ok := data["mean_score"]; !ok {
		t.Fatal("expected mean_score key in JSON")
	}
}

func TestUnknownModel(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary: "answer",
			Model:   model.ModelRef{Model: ""},
		},
	}
	r := Analyze(fs)
	if r.Models[0].Model != "unknown" {
		t.Fatalf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestMeanScore(t *testing.T) {
	fs := []model.Finding{
		{ID: "f1", Lab: "lab-1", Summary: "answer about X", Model: model.ModelRef{Model: "m1"}, Location: model.Location{PathTemplate: "What is X?"}},
		{ID: "f2", Lab: "lab-1", Summary: "answer about Y", Model: model.ModelRef{Model: "m2"}, Location: model.Location{PathTemplate: "What is Y?"}},
	}
	r := Analyze(fs)
	if r.MeanScore < 0 {
		t.Fatal("mean score should be non-negative")
	}
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}
