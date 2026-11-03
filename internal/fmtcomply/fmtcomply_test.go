package fmtcomply

import (
	"encoding/json"
	"testing"
)

func TestValidJSON(t *testing.T) {
	check := CheckFormat(`{"key": "value", "num": 42}`, FormatJSON)
	if !check.Valid {
		t.Error("expected valid JSON")
	}
	if check.StructuralScore != 100 {
		t.Errorf("expected score 100, got %.1f", check.StructuralScore)
	}
}

func TestInvalidJSON(t *testing.T) {
	check := CheckFormat(`{key: value}`, FormatJSON)
	if check.Valid {
		t.Error("expected invalid JSON")
	}
	if check.StructuralScore == 0 {
		t.Error("expected partial score for JSON-like content")
	}
}

func TestJSONInCodeBlock(t *testing.T) {
	response := "Here is the result:\n```json\n{\"a\": 1}\n```\n"
	check := CheckFormat(response, FormatJSON)
	if !check.Valid {
		t.Error("expected valid JSON extracted from code block")
	}
}

func TestValidXML(t *testing.T) {
	check := CheckFormat(`<root><item>hello</item></root>`, FormatXML)
	if !check.Valid {
		t.Error("expected valid XML")
	}
}

func TestMismatchedXMLTags(t *testing.T) {
	check := CheckFormat(`<root><item>hello</root>`, FormatXML)
	if check.Valid {
		t.Error("expected invalid XML for mismatched tags")
	}
	if len(check.Violations) == 0 {
		t.Error("expected violations for mismatched tags")
	}
}

func TestValidCSV(t *testing.T) {
	csv := "name,age,city\nAlice,30,NYC\nBob,25,LA\n"
	check := CheckFormat(csv, FormatCSV)
	if !check.Valid {
		t.Error("expected valid CSV")
	}
}

func TestInconsistentCSV(t *testing.T) {
	csv := "name,age,city\nAlice,30\nBob,25,LA\n"
	check := CheckFormat(csv, FormatCSV)
	if check.Valid {
		t.Error("expected invalid CSV for inconsistent columns")
	}
}

func TestValidNumberedList(t *testing.T) {
	list := "1. First item\n2. Second item\n3. Third item\n"
	check := CheckFormat(list, FormatNumbered)
	if !check.Valid {
		t.Error("expected valid numbered list")
	}
}

func TestNoNumberedList(t *testing.T) {
	check := CheckFormat("Just some plain text without numbers.", FormatNumbered)
	if check.Valid {
		t.Error("expected invalid for non-numbered text")
	}
}

func TestValidBulletedList(t *testing.T) {
	list := "- Item one\n- Item two\n- Item three\n"
	check := CheckFormat(list, FormatBulleted)
	if !check.Valid {
		t.Error("expected valid bulleted list")
	}
}

func TestValidCodeBlock(t *testing.T) {
	code := "```python\nprint('hello')\n```\n"
	check := CheckFormat(code, FormatCodeBlock)
	if !check.Valid {
		t.Error("expected valid code block")
	}
}

func TestUnclosedCodeBlock(t *testing.T) {
	code := "```python\nprint('hello')\n"
	check := CheckFormat(code, FormatCodeBlock)
	if check.Valid {
		t.Error("expected invalid for unclosed code block")
	}
}

func TestValidKeyValue(t *testing.T) {
	kv := "name: Alice\nage: 30\ncity: NYC\n"
	check := CheckFormat(kv, FormatKeyValue)
	if !check.Valid {
		t.Error("expected valid key-value format")
	}
}

func TestMarkdownWithHeadings(t *testing.T) {
	md := "# Title\n\nSome text with **bold** and `code`.\n"
	check := CheckFormat(md, FormatMarkdown)
	if !check.Valid {
		t.Error("expected valid markdown")
	}
}

func TestMarkdownWithoutHeadings(t *testing.T) {
	md := "Just plain text without any formatting."
	check := CheckFormat(md, FormatMarkdown)
	if check.Valid {
		t.Error("expected invalid for plain text as markdown")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	samples := []Sample{
		{Model: "model-a", Response: `{"key": "val"}`, RequestedFormat: FormatJSON},
		{Model: "model-a", Response: `{"key": "val2"}`, RequestedFormat: FormatJSON},
		{Model: "model-b", Response: `not json`, RequestedFormat: FormatJSON},
	}
	r := Analyze(samples)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestCompliance == "" {
		t.Error("expected best compliance to be set")
	}
}

func TestAnalyzeGrading(t *testing.T) {
	samples := []Sample{
		{Model: "good", Response: `{"a":1}`, RequestedFormat: FormatJSON},
	}
	r := Analyze(samples)
	if r.Models[0].Grade != "A+" {
		t.Errorf("expected A+ for perfect compliance, got %s", r.Models[0].Grade)
	}
}

func TestFormatTextOutput(t *testing.T) {
	samples := []Sample{
		{Model: "test", Response: `{"a":1}`, RequestedFormat: FormatJSON},
	}
	r := Analyze(samples)
	text := FormatText(r)
	if text == "" {
		t.Error("expected non-empty text output")
	}
	if !containsStr(text, "Format Compliance") {
		t.Error("expected header in text output")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	samples := []Sample{
		{Model: "test", Response: `{"a":1}`, RequestedFormat: FormatJSON},
	}
	r := Analyze(samples)
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var r2 Report
	if err := json.Unmarshal(data, &r2); err != nil {
		t.Fatal(err)
	}
	if len(r2.Models) != 1 {
		t.Errorf("expected 1 model after round-trip, got %d", len(r2.Models))
	}
}

func TestEmptySamples(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected 0 models for empty samples, got %d", len(r.Models))
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
