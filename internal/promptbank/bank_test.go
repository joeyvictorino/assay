package promptbank

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddAndGet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "prompts")
	b := New(dir)
	tmpl := Template{
		Name:               "sql-injection",
		Description:        "Test for SQL injection",
		Category:           "security",
		SystemPrompt:       "You are a security tester.",
		UserPromptTemplate: "Check {{target}} for SQL injection.",
		Variables: []Variable{
			{Name: "target", Description: "The target URL", Default: "http://localhost"},
		},
	}
	if err := b.Add(tmpl); err != nil {
		t.Fatal(err)
	}
	got, err := b.Get("sql-injection")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "sql-injection" {
		t.Fatalf("name=%s want sql-injection", got.Name)
	}
	if got.Category != "security" {
		t.Fatalf("category=%s", got.Category)
	}
	if len(got.Variables) != 1 || got.Variables[0].Name != "target" {
		t.Fatalf("variables=%v", got.Variables)
	}
}

func TestListEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	b := New(dir)
	templates, err := b.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 0 {
		t.Fatalf("expected empty list, got %d", len(templates))
	}
}

func TestListSorted(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "prompts")
	b := New(dir)
	for _, name := range []string{"zzz", "aaa", "mmm"} {
		if err := b.Add(Template{Name: name, Description: "test", Category: "test", SystemPrompt: "sys", UserPromptTemplate: "usr"}); err != nil {
			t.Fatal(err)
		}
	}
	templates, err := b.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 3 {
		t.Fatalf("len=%d want 3", len(templates))
	}
	if templates[0].Name != "aaa" || templates[1].Name != "mmm" || templates[2].Name != "zzz" {
		t.Fatalf("order: %s, %s, %s", templates[0].Name, templates[1].Name, templates[2].Name)
	}
}

func TestAddRequiresName(t *testing.T) {
	b := New(t.TempDir())
	if err := b.Add(Template{}); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestExportJSON(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "prompts")
	b := New(dir)
	b.Add(Template{Name: "test", Description: "d", Category: "c", SystemPrompt: "s", UserPromptTemplate: "u"})
	out, err := b.Export("json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"name": "test"`) {
		t.Fatalf("json output missing name: %s", out)
	}
}

func TestExportYAML(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "prompts")
	b := New(dir)
	b.Add(Template{
		Name: "t1", Description: "d", Category: "c",
		SystemPrompt: "s", UserPromptTemplate: "u",
		Variables: []Variable{{Name: "v1", Description: "desc", Default: "def"}},
	})
	out, err := b.Export("yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "name: t1") {
		t.Fatalf("yaml missing name: %s", out)
	}
	if !strings.Contains(out, "variables:") {
		t.Fatalf("yaml missing variables: %s", out)
	}
}

func TestExportUnknownFormat(t *testing.T) {
	b := New(t.TempDir())
	_, err := b.Export("xml")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestGetNonexistent(t *testing.T) {
	b := New(t.TempDir())
	_, err := b.Get("nope")
	if err == nil {
		t.Fatal("expected error for missing template")
	}
}

func TestListIgnoresNonJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not json"), 0o644)
	os.WriteFile(filepath.Join(dir, "valid.json"), []byte(`{"name":"valid","description":"d","category":"c","system_prompt":"s","user_prompt_template":"u"}`), 0o644)
	b := New(dir)
	templates, err := b.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 1 || templates[0].Name != "valid" {
		t.Fatalf("templates=%v", templates)
	}
}
