package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/promptbank"
)

func TestPromptBankAddAndShow(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	tmplFile := filepath.Join(dir, "tmpl.json")
	tmpl := promptbank.Template{
		Name:               "test-template",
		Description:        "A test template",
		Category:           "testing",
		SystemPrompt:       "You are a tester.",
		UserPromptTemplate: "Test {{target}}",
		Variables: []promptbank.Variable{
			{Name: "target", Description: "what to test", Default: "all"},
		},
	}
	data, _ := json.Marshal(tmpl)
	os.WriteFile(tmplFile, data, 0o644)

	var out, errb bytes.Buffer
	code := runPromptBank([]string{"add", "--file", tmplFile}, &out, &errb)
	if code != ExitPass {
		t.Fatalf("add exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "Added template") {
		t.Error("expected confirmation message")
	}

	out.Reset()
	errb.Reset()
	code = runPromptBank([]string{"show", "test-template"}, &out, &errb)
	if code != ExitPass {
		t.Fatalf("show exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "test-template") {
		t.Error("missing template name in show output")
	}
	if !strings.Contains(out.String(), "target") {
		t.Error("missing variable in show output")
	}
}

func TestPromptBankListEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var out, errb bytes.Buffer
	code := runPromptBank([]string{"list"}, &out, &errb)
	if code != ExitPass {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "No prompt templates") {
		t.Error("expected empty list message")
	}
}

func TestPromptBankExportJSON(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	bankDir := filepath.Join(dir, ".assay", "prompts")
	os.MkdirAll(bankDir, 0o755)
	tmpl := promptbank.Template{
		Name:               "t1",
		Description:        "d",
		Category:           "c",
		SystemPrompt:       "s",
		UserPromptTemplate: "u",
	}
	data, _ := json.MarshalIndent(tmpl, "", "  ")
	os.WriteFile(filepath.Join(bankDir, "t1.json"), append(data, '\n'), 0o644)

	var out, errb bytes.Buffer
	code := runPromptBank([]string{"export", "--format", "json"}, &out, &errb)
	if code != ExitPass {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), `"name": "t1"`) {
		t.Errorf("missing template in export: %s", out.String())
	}
}

func TestPromptBankNoSubcommand(t *testing.T) {
	var out, errb bytes.Buffer
	code := runPromptBank([]string{}, &out, &errb)
	if code != ExitError {
		t.Fatalf("expected error exit, got %d", code)
	}
}

func TestPromptBankUnknownSubcommand(t *testing.T) {
	var out, errb bytes.Buffer
	code := runPromptBank([]string{"nope"}, &out, &errb)
	if code != ExitError {
		t.Fatalf("expected error exit, got %d", code)
	}
}

func TestPromptBankShowMissingName(t *testing.T) {
	var out, errb bytes.Buffer
	code := runPromptBank([]string{"show"}, &out, &errb)
	if code != ExitError {
		t.Fatalf("expected error exit, got %d", code)
	}
}

func TestPromptBankAddMissingFile(t *testing.T) {
	var out, errb bytes.Buffer
	code := runPromptBank([]string{"add"}, &out, &errb)
	if code != ExitError {
		t.Fatalf("expected error exit, got %d", code)
	}
}
