package prompttemplate

import (
	"strings"
	"testing"
)

const testInput = `---
name: safety-probe
category: adversarial
description: Test model safety boundaries
params: topic, severity
pattern: Explain how to {{topic}} with {{severity}} level detail
---
name: factual-check
category: grounding
description: Verify factual claims
params: claim
pattern: Is the following claim true? {{claim}}
---
name: creative-write
category: creative
description: Generate creative content
params: style, subject
pattern: Write a {{style}} about {{subject}}
`

func TestParse(t *testing.T) {
	lib, err := Parse(testInput)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Templates) != 3 {
		t.Fatalf("expected 3 templates, got %d", len(lib.Templates))
	}
	if lib.Templates[0].Name != "safety-probe" {
		t.Fatalf("expected safety-probe, got %s", lib.Templates[0].Name)
	}
	if len(lib.Templates[0].Params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(lib.Templates[0].Params))
	}
}

func TestParseEmpty(t *testing.T) {
	lib, err := Parse("")
	if err != nil {
		t.Fatal(err)
	}
	if len(lib.Templates) != 0 {
		t.Fatalf("expected 0 templates, got %d", len(lib.Templates))
	}
}

func TestRender(t *testing.T) {
	tmpl := Template{
		Name:    "test",
		Pattern: "Hello {{name}}, you are {{age}}",
		Params:  []string{"name", "age"},
	}
	result := Render(tmpl, map[string]string{"name": "Alice", "age": "30"})
	if result.Rendered != "Hello Alice, you are 30" {
		t.Fatalf("unexpected render: %s", result.Rendered)
	}
	if len(result.MissingParams) != 0 {
		t.Fatalf("expected no missing params, got %v", result.MissingParams)
	}
}

func TestRenderMissing(t *testing.T) {
	tmpl := Template{
		Name:    "test",
		Pattern: "{{a}} and {{b}}",
		Params:  []string{"a", "b"},
	}
	result := Render(tmpl, map[string]string{"a": "hello"})
	if len(result.MissingParams) != 1 || result.MissingParams[0] != "b" {
		t.Fatalf("expected missing param b, got %v", result.MissingParams)
	}
}

func TestCategorize(t *testing.T) {
	lib, _ := Parse(testInput)
	cats := Categorize(lib)
	if len(cats) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(cats))
	}
	if len(cats["adversarial"]) != 1 {
		t.Fatal("expected 1 adversarial template")
	}
}

func TestAnalyze(t *testing.T) {
	lib, _ := Parse(testInput)
	r := Analyze(lib)
	if r.Summary.TotalTemplates != 3 {
		t.Fatalf("expected 3 templates, got %d", r.Summary.TotalTemplates)
	}
	if r.Summary.TotalParams != 5 {
		t.Fatalf("expected 5 total params, got %d", r.Summary.TotalParams)
	}
}

func TestFormatText(t *testing.T) {
	lib, _ := Parse(testInput)
	r := Analyze(lib)
	out := FormatText(r)
	if !strings.Contains(out, "Prompt Template Library") {
		t.Fatal("expected header")
	}
	if !strings.Contains(out, "safety-probe") {
		t.Fatal("expected template name")
	}
}

func TestFormatJSON(t *testing.T) {
	lib, _ := Parse(testInput)
	r := Analyze(lib)
	out := FormatJSON(r)
	if !strings.Contains(out, `"total_templates"`) {
		t.Fatal("expected total_templates in JSON")
	}
}

func TestNewLibrary(t *testing.T) {
	lib := NewLibrary()
	if lib == nil {
		t.Fatal("expected non-nil library")
	}
	if len(lib.Templates) != 0 {
		t.Fatal("expected empty templates")
	}
}
