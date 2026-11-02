package report

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestHTML_ContainsDoctype(t *testing.T) {
	rr := model.RunReport{RunID: "test-run"}
	h := HTML(rr, nil, nil)
	if !strings.HasPrefix(h, "<!DOCTYPE html>") {
		t.Error("HTML output should start with <!DOCTYPE html>")
	}
}

func TestHTML_ContainsRunID(t *testing.T) {
	rr := model.RunReport{RunID: "abc-123"}
	h := HTML(rr, nil, nil)
	if !strings.Contains(h, "abc-123") {
		t.Error("HTML should contain the run ID")
	}
}

func TestHTML_ContainsStyles(t *testing.T) {
	rr := model.RunReport{RunID: "styled"}
	h := HTML(rr, nil, nil)
	if !strings.Contains(h, "<style>") {
		t.Error("HTML should contain inline styles")
	}
	if !strings.Contains(h, "prefers-color-scheme: dark") {
		t.Error("HTML should include dark mode styles")
	}
}

func TestHTML_TablesRendered(t *testing.T) {
	rr := model.RunReport{
		RunID: "table-test",
		Models: []model.ModelResult{
			{
				Ref:      model.ModelRef{Model: "test-model"},
				Findings: map[model.FindingState]int{model.StateValidated: 3, model.StateTheorized: 1},
			},
		},
	}
	h := HTML(rr, nil, nil)
	if !strings.Contains(h, "<table>") {
		t.Error("HTML should contain table elements")
	}
	if !strings.Contains(h, "<th>") {
		t.Error("HTML should contain table headers")
	}
}

func TestHTML_FindingsIncluded(t *testing.T) {
	rr := model.RunReport{RunID: "findings-test"}
	findings := []model.Finding{
		{ID: "f1", DedupKey: "k1", State: model.StateValidated, Severity: model.SevHigh, Model: model.ModelRef{Model: "m1"}},
	}
	h := HTML(rr, findings, nil)
	if !strings.Contains(h, "findings") || !strings.Contains(h, "Findings") {
		t.Error("HTML should contain findings section")
	}
}

func TestMarkdownToHTML_Headings(t *testing.T) {
	html := markdownToHTML("# Title\n\n## Section\n\nParagraph text.\n")
	if !strings.Contains(html, "<h1>Title</h1>") {
		t.Error("should convert h1")
	}
	if !strings.Contains(html, "<h2>Section</h2>") {
		t.Error("should convert h2")
	}
	if !strings.Contains(html, "<p>Paragraph text.</p>") {
		t.Error("should wrap paragraph")
	}
}

func TestMarkdownToHTML_CodeBlock(t *testing.T) {
	md := "```\ncode here\n```\n"
	html := markdownToHTML(md)
	if !strings.Contains(html, "<pre><code>") {
		t.Error("should create code block")
	}
	if !strings.Contains(html, "code here") {
		t.Error("should preserve code content")
	}
}

func TestMarkdownToHTML_List(t *testing.T) {
	md := "- item one\n- item two\n"
	html := markdownToHTML(md)
	if !strings.Contains(html, "<ul>") {
		t.Error("should create unordered list")
	}
	if !strings.Contains(html, "<li>item one</li>") {
		t.Error("should create list items")
	}
}

func TestMarkdownToHTML_Blockquote(t *testing.T) {
	md := "> quoted text\n"
	html := markdownToHTML(md)
	if !strings.Contains(html, "<blockquote>") {
		t.Error("should create blockquote")
	}
}

func TestInlineFormat_Bold(t *testing.T) {
	got := inlineFormat("this is **bold** text")
	if !strings.Contains(got, "<strong>bold</strong>") {
		t.Errorf("expected bold, got %q", got)
	}
}

func TestInlineFormat_Code(t *testing.T) {
	got := inlineFormat("use `code` here")
	if !strings.Contains(got, "<code>code</code>") {
		t.Errorf("expected code, got %q", got)
	}
}
