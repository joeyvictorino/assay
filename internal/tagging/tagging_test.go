package tagging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(id, class, summary, sev string) model.Finding {
	return model.Finding{
		ID:       id,
		Class:    model.Class(class),
		Severity: model.Severity(sev),
		Summary:  summary,
		Model:    model.ModelRef{Provider: "openai", Model: "gpt-4"},
		Lab:      "lab1",
	}
}

func TestApplyTagsByClass(t *testing.T) {
	rules := []TagRule{
		{Tag: "injection", Classes: []string{"sqli", "xss-reflected", "xss-stored"}},
	}
	fs := []model.Finding{
		finding("f1", "sqli", "SQL injection found", "high"),
		finding("f2", "idor", "Insecure direct object ref", "medium"),
	}
	tagged := ApplyTags(fs, rules)
	if len(tagged[0].Tags) != 1 || tagged[0].Tags[0] != "injection" {
		t.Errorf("f1 tags = %v, want [injection]", tagged[0].Tags)
	}
	if len(tagged[1].Tags) != 0 {
		t.Errorf("f2 tags = %v, want []", tagged[1].Tags)
	}
}

func TestApplyTagsByKeyword(t *testing.T) {
	rules := []TagRule{
		{Tag: "web", Keywords: []string{"XSS", "cross-site"}},
	}
	fs := []model.Finding{
		finding("f1", "xss-reflected", "Cross-site scripting in form", "medium"),
		finding("f2", "sqli", "SQL injection", "high"),
	}
	tagged := ApplyTags(fs, rules)
	if len(tagged[0].Tags) != 1 || tagged[0].Tags[0] != "web" {
		t.Errorf("f1 tags = %v, want [web]", tagged[0].Tags)
	}
	if len(tagged[1].Tags) != 0 {
		t.Errorf("f2 should not be tagged")
	}
}

func TestMultipleTags(t *testing.T) {
	rules := []TagRule{
		{Tag: "injection", Classes: []string{"sqli"}},
		{Tag: "critical-class", Keywords: []string{"SQL injection"}},
	}
	fs := []model.Finding{
		finding("f1", "sqli", "SQL injection found", "critical"),
	}
	tagged := ApplyTags(fs, rules)
	if len(tagged[0].Tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(tagged[0].Tags), tagged[0].Tags)
	}
}

func TestNoDuplicateTags(t *testing.T) {
	rules := []TagRule{
		{Tag: "injection", Classes: []string{"sqli"}},
		{Tag: "injection", Keywords: []string{"SQL"}},
	}
	fs := []model.Finding{
		finding("f1", "sqli", "SQL injection", "high"),
	}
	tagged := ApplyTags(fs, rules)
	count := 0
	for _, tag := range tagged[0].Tags {
		if tag == "injection" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("tag 'injection' appears %d times, want 1", count)
	}
}

func TestFilterByTag(t *testing.T) {
	tagged := []TaggedFinding{
		{ID: "f1", Tags: []string{"injection", "web"}},
		{ID: "f2", Tags: []string{"auth"}},
		{ID: "f3", Tags: []string{"injection"}},
	}
	filtered := FilterByTag(tagged, "injection")
	if len(filtered) != 2 {
		t.Errorf("expected 2, got %d", len(filtered))
	}
}

func TestFilterByTagCaseInsensitive(t *testing.T) {
	tagged := []TaggedFinding{
		{ID: "f1", Tags: []string{"Injection"}},
	}
	filtered := FilterByTag(tagged, "injection")
	if len(filtered) != 1 {
		t.Error("case-insensitive filter failed")
	}
}

func TestAnalyze(t *testing.T) {
	rules := []TagRule{
		{Tag: "injection", Classes: []string{"sqli"}},
	}
	fs := []model.Finding{
		finding("f1", "sqli", "SQL injection", "high"),
		finding("f2", "idor", "IDOR", "medium"),
	}
	r := Analyze(fs, rules)
	if r.TotalFindings != 2 {
		t.Errorf("TotalFindings = %d, want 2", r.TotalFindings)
	}
	if r.TotalTagged != 1 {
		t.Errorf("TotalTagged = %d, want 1", r.TotalTagged)
	}
	if r.TotalUntagged != 1 {
		t.Errorf("TotalUntagged = %d, want 1", r.TotalUntagged)
	}
	if r.TagCounts["injection"] != 1 {
		t.Errorf("injection count = %d, want 1", r.TagCounts["injection"])
	}
}

func TestLoadRules(t *testing.T) {
	content := `rules:
  - tag: injection
    classes: [sqli, xss-reflected]
  - tag: auth
    keywords: [authentication, authorization]
`
	dir := t.TempDir()
	path := filepath.Join(dir, "tags.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rules, err := LoadRules(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rules))
	}
	if rules[0].Tag != "injection" {
		t.Errorf("first rule tag = %q, want injection", rules[0].Tag)
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil, nil)
	if r.TotalFindings != 0 {
		t.Error("expected 0 findings")
	}
}

func TestFormatText(t *testing.T) {
	rules := []TagRule{{Tag: "injection", Classes: []string{"sqli"}}}
	fs := []model.Finding{finding("f1", "sqli", "SQL injection", "high")}
	r := Analyze(fs, rules)
	text := FormatText(r)
	if !strings.Contains(text, "Findings Tagging Report") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "injection") {
		t.Error("missing tag name")
	}
}

func TestFormatJSON(t *testing.T) {
	rules := []TagRule{{Tag: "web", Keywords: []string{"XSS"}}}
	fs := []model.Finding{finding("f1", "xss-reflected", "XSS found", "medium")}
	r := Analyze(fs, rules)
	j := FormatJSON(r)
	if !strings.Contains(j, "tag_counts") {
		t.Error("missing tag_counts in JSON")
	}
}

func TestClassSubstringMatch(t *testing.T) {
	rules := []TagRule{
		{Tag: "xss", Classes: []string{"xss"}},
	}
	fs := []model.Finding{
		finding("f1", "xss-reflected", "Reflected XSS", "medium"),
		finding("f2", "xss-stored", "Stored XSS", "high"),
	}
	tagged := ApplyTags(fs, rules)
	if len(tagged[0].Tags) != 1 {
		t.Errorf("xss-reflected should match 'xss' class rule, got tags: %v", tagged[0].Tags)
	}
	if len(tagged[1].Tags) != 1 {
		t.Errorf("xss-stored should match 'xss' class rule, got tags: %v", tagged[1].Tags)
	}
}
