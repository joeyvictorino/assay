package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCreatesProject(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "myproject")

	var stdout, stderr bytes.Buffer
	code := runInit([]string{"--dir", target, "--template", "strict"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass, got %d; stderr: %s", code, stderr.String())
	}

	for _, name := range []string{"assay.yaml", "manifests/example-tool.json", ".assay"} {
		p := filepath.Join(target, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist: %v", name, err)
		}
	}

	out := stdout.String()
	if !strings.Contains(out, "Initialized") {
		t.Errorf("output should contain 'Initialized', got: %s", out)
	}
	if !strings.Contains(out, "strict") {
		t.Errorf("output should mention template name, got: %s", out)
	}
}

func TestInitRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "assay.yaml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runInit([]string{"--dir", dir}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError when files exist, got %d", code)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("stderr should mention existing file: %s", stderr.String())
	}
}

func TestInitForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "assay.yaml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runInit([]string{"--dir", dir, "--force"}, &stdout, &stderr)
	if code != ExitPass {
		t.Fatalf("expected ExitPass with --force, got %d; stderr: %s", code, stderr.String())
	}

	data, _ := os.ReadFile(filepath.Join(dir, "assay.yaml"))
	if string(data) == "old" {
		t.Error("file should have been overwritten")
	}
}

func TestInitBadTemplate(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runInit([]string{"--dir", filepath.Join(dir, "x"), "--template", "nonexistent"}, &stdout, &stderr)
	if code != ExitError {
		t.Errorf("expected ExitError for bad template, got %d", code)
	}
}
