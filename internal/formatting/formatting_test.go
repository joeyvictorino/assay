package formatting

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(mdl, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mdl},
		Summary: summary,
		Lab:     "test-lab",
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatal("expected no models")
	}
}

func TestAnalyzeSinglePlain(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Just plain text response.")}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].FormattingScore <= 0 {
		t.Error("expected positive score")
	}
}

func TestFormattedScoresHigher(t *testing.T) {
	plain := []model.Finding{mkFinding("m1", "Just a plain answer with no formatting.")}
	formatted := []model.Finding{mkFinding("m1", "## Overview\n\nHere are the key points:\n\n- First item in list\n- Second item\n- Third item\n\n### Details\n\n```python\nprint('hello')\n```\n\nThe **bold** and *italic* formatting helps readability.")}
	rp := Analyze(plain)
	rf := Analyze(formatted)
	if rf.Models[0].FormattingScore <= rp.Models[0].FormattingScore {
		t.Errorf("formatted (%.1f) should score higher than plain (%.1f)", rf.Models[0].FormattingScore, rp.Models[0].FormattingScore)
	}
}

func TestMarkdownScore(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "The **bold** text and *italic* text with `code` and [links](url) improve readability.")}
	r := Analyze(fs)
	if r.Models[0].Detail.MarkdownScore < 0.3 {
		t.Errorf("expected higher markdown score, got %.2f", r.Models[0].Detail.MarkdownScore)
	}
}

func TestCodeBlockScore(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Here is an example:\n\n```python\ndef hello():\n    print('world')\n```\n\nAnd another:\n\n```go\nfmt.Println(\"hi\")\n```")}
	r := Analyze(fs)
	if r.Models[0].Detail.CodeBlockScore < 0.5 {
		t.Errorf("expected higher code block score, got %.2f", r.Models[0].Detail.CodeBlockScore)
	}
}

func TestListScore(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Items:\n- First thing\n- Second thing\n- Third thing\n- Fourth thing\n1. Step one\n2. Step two")}
	r := Analyze(fs)
	if r.Models[0].Detail.ListScore < 0.5 {
		t.Errorf("expected higher list score, got %.2f", r.Models[0].Detail.ListScore)
	}
}

func TestHeadingScore(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "\n# Main Title\n\nSome content here.\n\n## Section Two\n\nMore content.\n\n### Subsection\n\nDetails here.")}
	r := Analyze(fs)
	if r.Models[0].Detail.HeadingScore < 0.5 {
		t.Errorf("expected higher heading score, got %.2f", r.Models[0].Detail.HeadingScore)
	}
}

func TestTableScore(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "| Name | Value |\n|---|---|\n| A | 1 |\n| B | 2 |")}
	r := Analyze(fs)
	if r.Models[0].Detail.TableScore < 0.3 {
		t.Errorf("expected higher table score, got %.2f", r.Models[0].Detail.TableScore)
	}
}

func TestPoorFormattingPenalty(t *testing.T) {
	good := []model.Finding{mkFinding("m1", "Well structured response with proper spacing.")}
	bad := []model.Finding{mkFinding("m1", "Bad formatting\n\n\n\n\n\n\n\nwith excessive whitespace........and dots........everywhere")}
	rg := Analyze(good)
	rb := Analyze(bad)
	if rb.Models[0].FormattingScore >= rg.Models[0].FormattingScore {
		t.Error("poor formatting should score lower")
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "\n## Title\n\nWith **formatting** and:\n- lists\n- items"),
		mkFinding("m2", "Plain text only."),
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best/worst set")
	}
}

func TestGradeA(t *testing.T) {
	g := scoreToGrade(95)
	if g != "A" {
		t.Errorf("expected A, got %s", g)
	}
}

func TestGradeB(t *testing.T) {
	g := scoreToGrade(85)
	if g != "B" {
		t.Errorf("expected B, got %s", g)
	}
}

func TestGradeC(t *testing.T) {
	g := scoreToGrade(75)
	if g != "C" {
		t.Errorf("expected C, got %s", g)
	}
}

func TestGradeD(t *testing.T) {
	g := scoreToGrade(65)
	if g != "D" {
		t.Errorf("expected D, got %s", g)
	}
}

func TestGradeF(t *testing.T) {
	g := scoreToGrade(45)
	if g != "F" {
		t.Errorf("expected F, got %s", g)
	}
}

func TestScoreBounded(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", strings.Repeat("\n## Heading\n\n- list item\n```code```\n**bold** *italic*\n| col |---|val |\n", 20))}
	r := Analyze(fs)
	if r.Models[0].FormattingScore < 0 || r.Models[0].FormattingScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].FormattingScore)
	}
}

func TestScoreBoundedLow(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", strings.Repeat("........\n\n\n\n========", 10))}
	r := Analyze(fs)
	if r.Models[0].FormattingScore < 0 || r.Models[0].FormattingScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].FormattingScore)
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Response with **formatting**.")}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Formatting") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "m1") {
		t.Error("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No formatting data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Test response.")}
	r := Analyze(fs)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Error("expected models key")
	}
}

func TestFormatJSONRoundtrip(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestMeanScore(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "\n## Heading\n\nWith **bold** and lists:\n- item"),
		mkFinding("m2", "Another model response."),
	}
	r := Analyze(fs)
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestLabCount(t *testing.T) {
	fs := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test2", Lab: "lab2"},
	}
	r := Analyze(fs)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestUnknownModel(t *testing.T) {
	fs := []model.Finding{{Summary: "test", Lab: "lab1"}}
	r := Analyze(fs)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown model, got %s", r.Models[0].Model)
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback",
		Evidence: []model.Evidence{
			{Marker: "marker1"},
			{Marker: "marker2"},
		},
	}
	et := evidenceText(f)
	if !strings.Contains(et, "marker1") {
		t.Error("expected marker in evidence text")
	}
}

func TestEvidenceFallback(t *testing.T) {
	f := model.Finding{Summary: "my summary"}
	et := evidenceText(f)
	if et != "my summary" {
		t.Errorf("expected summary fallback, got %q", et)
	}
}

func TestCountIndicators(t *testing.T) {
	text := "**bold** and **more bold**"
	c := countIndicators(text, []string{"**"})
	if c != 4 {
		t.Errorf("expected 4, got %d", c)
	}
}

func TestCountLines(t *testing.T) {
	c := countLines("line1\nline2\nline3")
	if c != 3 {
		t.Errorf("expected 3, got %d", c)
	}
}

func TestCountLinesEmpty(t *testing.T) {
	c := countLines("")
	if c != 0 {
		t.Errorf("expected 0, got %d", c)
	}
}

func TestWhitespaceScore(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "First line with content\nSecond line here\nThird line of text\nFourth line continues\nFifth line ends")}
	r := Analyze(fs)
	if r.Models[0].Detail.WhitespaceScore < 0.5 {
		t.Errorf("expected decent whitespace score, got %.2f", r.Models[0].Detail.WhitespaceScore)
	}
}
