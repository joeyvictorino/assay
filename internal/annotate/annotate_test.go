package annotate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func makeFindingsFile(t *testing.T, dir string) string {
	t.Helper()
	findings := []map[string]any{
		{"id": "f1", "class": "injection", "severity": "high", "summary": "SQL injection"},
		{"id": "f2", "class": "xss", "severity": "medium", "summary": "Reflected XSS"},
		{"id": "f3", "class": "ssrf", "severity": "critical", "summary": "SSRF via redirect"},
	}
	data, _ := json.MarshalIndent(findings, "", "  ")
	path := filepath.Join(dir, "findings.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAndAnnotate(t *testing.T) {
	dir := t.TempDir()
	path := makeFindingsFile(t, dir)

	af, err := LoadAnnotated(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(af.Findings) != 3 {
		t.Fatalf("findings = %d, want 3", len(af.Findings))
	}

	err = AddAnnotation(af, "f1", "analyst@example.com", "Confirmed exploitable", []string{"verified"}, "confirmed")
	if err != nil {
		t.Fatal(err)
	}
	if len(af.Annotations) != 1 {
		t.Fatalf("annotations = %d, want 1", len(af.Annotations))
	}

	outPath := filepath.Join(dir, "annotated.json")
	if err := Save(af, outPath); err != nil {
		t.Fatal(err)
	}

	af2, err := LoadAnnotated(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(af2.Annotations) != 1 {
		t.Fatalf("reloaded annotations = %d, want 1", len(af2.Annotations))
	}
	if af2.Annotations[0].FindingID != "f1" {
		t.Errorf("finding_id = %q, want f1", af2.Annotations[0].FindingID)
	}
}

func TestAnnotateUnknownFinding(t *testing.T) {
	dir := t.TempDir()
	path := makeFindingsFile(t, dir)

	af, err := LoadAnnotated(path)
	if err != nil {
		t.Fatal(err)
	}

	err = AddAnnotation(af, "nonexistent", "test", "note", nil, "")
	if err == nil {
		t.Fatal("expected error for unknown finding")
	}
}

func TestListAnnotations(t *testing.T) {
	dir := t.TempDir()
	path := makeFindingsFile(t, dir)

	af, _ := LoadAnnotated(path)
	_ = AddAnnotation(af, "f1", "a", "note1", nil, "")
	_ = AddAnnotation(af, "f2", "b", "note2", nil, "")
	_ = AddAnnotation(af, "f1", "c", "note3", nil, "")

	all := ListAnnotations(af, "")
	if len(all) != 3 {
		t.Errorf("all = %d, want 3", len(all))
	}

	f1Only := ListAnnotations(af, "f1")
	if len(f1Only) != 2 {
		t.Errorf("f1 = %d, want 2", len(f1Only))
	}
}

func TestSummary(t *testing.T) {
	dir := t.TempDir()
	path := makeFindingsFile(t, dir)

	af, _ := LoadAnnotated(path)
	_ = AddAnnotation(af, "f1", "alice", "note", nil, "confirmed")
	_ = AddAnnotation(af, "f2", "bob", "note", nil, "false_positive")

	s := Summary(af)
	if s["total_annotations"].(int) != 2 {
		t.Errorf("total = %v, want 2", s["total_annotations"])
	}
	if s["annotated_findings"].(int) != 2 {
		t.Errorf("annotated = %v, want 2", s["annotated_findings"])
	}
}

func TestFormatAnnotationsText(t *testing.T) {
	dir := t.TempDir()
	path := makeFindingsFile(t, dir)

	af, _ := LoadAnnotated(path)
	_ = AddAnnotation(af, "f1", "analyst", "looks bad", []string{"urgent"}, "confirmed")

	text := FormatAnnotationsText(af.Annotations)
	if text == "" {
		t.Error("should not be empty")
	}
}

func TestFormatEmpty(t *testing.T) {
	text := FormatAnnotationsText(nil)
	if text != "No annotations.\n" {
		t.Errorf("unexpected text: %q", text)
	}
}
