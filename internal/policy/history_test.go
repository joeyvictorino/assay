package policy

import (
	"strings"
	"testing"
)

func TestBuildHistoryThreeVersions(t *testing.T) {
	minYAML, _ := GenerateTemplate("minimal")
	stdYAML, _ := GenerateTemplate("standard")
	strictYAML, _ := GenerateTemplate("strict")

	v1, err := Parse([]byte(minYAML))
	if err != nil {
		t.Fatal(err)
	}
	v2, err := Parse([]byte(stdYAML))
	if err != nil {
		t.Fatal(err)
	}
	v3, err := Parse([]byte(strictYAML))
	if err != nil {
		t.Fatal(err)
	}

	entries := BuildHistory([]*Engine{v1, v2, v3})
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].Version != 1 {
		t.Errorf("first entry version = %d, want 1", entries[0].Version)
	}
	if len(entries[0].Changes) != 1 || entries[0].Changes[0] != "initial version" {
		t.Errorf("first entry should be initial version, got %v", entries[0].Changes)
	}

	if entries[1].Version != 2 {
		t.Errorf("second entry version = %d, want 2", entries[1].Version)
	}
	if len(entries[1].Changes) == 0 || entries[1].Changes[0] == "no changes" {
		t.Error("v1->v2 should have changes")
	}

	if entries[2].Version != 3 {
		t.Errorf("third entry version = %d, want 3", entries[2].Version)
	}
	if len(entries[2].Changes) == 0 || entries[2].Changes[0] == "no changes" {
		t.Error("v2->v3 should have changes")
	}
}

func TestBuildHistoryEmpty(t *testing.T) {
	entries := BuildHistory(nil)
	if entries != nil {
		t.Errorf("expected nil, got %v", entries)
	}
}

func TestBuildHistorySingleVersion(t *testing.T) {
	yaml, _ := GenerateTemplate("minimal")
	eng, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatal(err)
	}
	entries := BuildHistory([]*Engine{eng})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Changes[0] != "initial version" {
		t.Errorf("expected initial version, got %v", entries[0].Changes)
	}
}

func TestBuildHistoryIdenticalVersions(t *testing.T) {
	yaml, _ := GenerateTemplate("minimal")
	v1, _ := Parse([]byte(yaml))
	v2, _ := Parse([]byte(yaml))
	entries := BuildHistory([]*Engine{v1, v2})
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[1].Changes[0] != "no changes" {
		t.Errorf("identical versions should show no changes, got %v", entries[1].Changes)
	}
}

func TestFormatHistoryText(t *testing.T) {
	entries := []HistoryEntry{
		{Version: 1, Hash: "abcdef123456789000", Changes: []string{"initial version"}},
		{Version: 2, Hash: "fedcba987654321000", Changes: []string{"rules added: r1, r2", "agents added: a1"}},
	}
	text := FormatHistoryText(entries)
	if !strings.Contains(text, "v1") {
		t.Error("should contain v1")
	}
	if !strings.Contains(text, "v2") {
		t.Error("should contain v2")
	}
	if !strings.Contains(text, "initial version") {
		t.Error("should contain initial version")
	}
	if !strings.Contains(text, "rules added: r1, r2") {
		t.Error("should contain rules added")
	}
	if !strings.Contains(text, "abcdef123456") {
		t.Error("should contain truncated hash")
	}
}

func TestHistoryHashesPopulated(t *testing.T) {
	minYAML, _ := GenerateTemplate("minimal")
	stdYAML, _ := GenerateTemplate("standard")
	v1, _ := Parse([]byte(minYAML))
	v2, _ := Parse([]byte(stdYAML))

	entries := BuildHistory([]*Engine{v1, v2})
	for _, e := range entries {
		if e.Hash == "" {
			t.Errorf("entry v%d has empty hash", e.Version)
		}
	}
}
