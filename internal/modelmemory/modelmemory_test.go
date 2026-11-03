package modelmemory

import (
	"strings"
	"testing"
)

func TestEmptyTurns(t *testing.T) {
	r := Analyze(nil)
	if r.Summary.RetentionScore != 100 {
		t.Fatalf("expected score 100, got %d", r.Summary.RetentionScore)
	}
}

func TestSingleTurn(t *testing.T) {
	r := Analyze([]Turn{{Role: "user", Content: "Hello", Index: 0}})
	if r.Summary.TotalTurns != 1 {
		t.Fatalf("expected 1 turn, got %d", r.Summary.TotalTurns)
	}
}

func TestFactRetention(t *testing.T) {
	turns := []Turn{
		{Role: "user", Content: "My name is Alice and I work at Globex Corporation", Index: 0},
		{Role: "assistant", Content: "Hello Alice! Nice to hear you work at Globex Corporation.", Index: 1},
		{Role: "user", Content: "What's my name?", Index: 2},
		{Role: "assistant", Content: "Your name is Alice and you work at Globex Corporation.", Index: 3},
	}
	r := Analyze(turns)
	if r.Summary.FactsIntroduced == 0 {
		t.Fatal("expected facts to be introduced")
	}
	if r.Summary.FactsRetained == 0 {
		t.Fatal("expected facts to be retained")
	}
}

func TestInstructionPersistence(t *testing.T) {
	turns := []Turn{
		{Role: "user", Content: "From now on, always respond in uppercase format", Index: 0},
		{Role: "assistant", Content: "I'll respond in uppercase format going forward.", Index: 1},
		{Role: "user", Content: "Tell me about cats", Index: 2},
		{Role: "assistant", Content: "CATS ARE WONDERFUL PETS in uppercase format as requested.", Index: 3},
	}
	r := Analyze(turns)
	if r.Summary.InstructionsGiven == 0 {
		t.Fatal("expected instructions to be detected")
	}
}

func TestReferenceTracking(t *testing.T) {
	turns := []Turn{
		{Role: "user", Content: "Hello", Index: 0},
		{Role: "assistant", Content: "Hi there", Index: 1},
		{Role: "user", Content: "More", Index: 2},
		{Role: "assistant", Content: "It was mentioned that they did this and that, as mentioned above. Those items and these things, the previous point, as mentioned.", Index: 3},
	}
	r := Analyze(turns)
	found := false
	for _, f := range r.Findings {
		if f.Category == "reference-track" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected reference tracking finding for pronoun-heavy turn")
	}
}

func TestRetentionScoreDecreasesWithoutRetention(t *testing.T) {
	turns := []Turn{
		{Role: "user", Content: "My name is Bartholomew and I live in Constantinople", Index: 0},
		{Role: "assistant", Content: "Okay, understood.", Index: 1},
		{Role: "user", Content: "What did I tell you?", Index: 2},
		{Role: "assistant", Content: "I don't recall the specifics.", Index: 3},
	}
	r := Analyze(turns)
	if r.Summary.RetentionScore >= 80 {
		t.Fatalf("expected low retention score, got %d", r.Summary.RetentionScore)
	}
}

func TestFormatTextContainsHeader(t *testing.T) {
	r := Analyze(nil)
	out := FormatText(r)
	if !strings.Contains(out, "Context Retention Analysis") {
		t.Fatal("expected header")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]Turn{{Role: "user", Content: "test", Index: 0}})
	out := FormatJSON(r)
	if !strings.Contains(out, "retention_score") {
		t.Fatal("expected JSON field retention_score")
	}
}

func TestNoFactsNoInstructions(t *testing.T) {
	turns := []Turn{
		{Role: "user", Content: "Hello", Index: 0},
		{Role: "assistant", Content: "Hi", Index: 1},
	}
	r := Analyze(turns)
	if r.Summary.RetentionScore != 100 {
		t.Fatalf("expected 100 retention when no facts/instructions, got %d", r.Summary.RetentionScore)
	}
}
