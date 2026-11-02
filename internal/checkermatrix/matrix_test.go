package checkermatrix

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestBuildEmpty(t *testing.T) {
	m := Build(nil)
	if len(m.Checkers) != 0 || len(m.Models) != 0 {
		t.Error("empty findings should produce empty matrix")
	}
}

func TestBuildSingleFinding(t *testing.T) {
	findings := []model.Finding{
		{Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh},
	}
	m := Build(findings)
	if len(m.Checkers) != 1 || m.Checkers[0] != "sqli" {
		t.Errorf("checkers = %v, want [sqli]", m.Checkers)
	}
	if len(m.Models) != 1 || m.Models[0] != "gpt-4" {
		t.Errorf("models = %v, want [gpt-4]", m.Models)
	}
	cell := m.Matrix["sqli:gpt-4"]
	if !cell.Ran || cell.Count != 1 {
		t.Errorf("cell = %+v, want ran=true count=1", cell)
	}
}

func TestBuildMultipleFindings(t *testing.T) {
	findings := []model.Finding{
		{Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh},
		{Lab: "sqli", Model: model.ModelRef{Model: "claude-3"}, Severity: model.SevMedium},
		{Lab: "xss", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevLow},
		{Lab: "xss", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevCritical},
	}
	m := Build(findings)
	if len(m.Checkers) != 2 {
		t.Errorf("checkers count = %d, want 2", len(m.Checkers))
	}
	if len(m.Models) != 2 {
		t.Errorf("models count = %d, want 2", len(m.Models))
	}

	xssGpt := m.Matrix["xss:gpt-4"]
	if xssGpt.Count != 2 {
		t.Errorf("xss:gpt-4 count = %d, want 2", xssGpt.Count)
	}

	_, hasSqliClaude := m.Matrix["sqli:claude-3"]
	if !hasSqliClaude {
		t.Error("missing sqli:claude-3 cell")
	}

	_, hasXssClaude := m.Matrix["xss:claude-3"]
	if hasXssClaude {
		t.Error("xss:claude-3 should not exist")
	}
}

func TestBuildSorted(t *testing.T) {
	findings := []model.Finding{
		{Lab: "zzz", Model: model.ModelRef{Model: "beta"}, Severity: model.SevLow},
		{Lab: "aaa", Model: model.ModelRef{Model: "alpha"}, Severity: model.SevHigh},
	}
	m := Build(findings)
	if m.Checkers[0] != "aaa" || m.Checkers[1] != "zzz" {
		t.Errorf("checkers not sorted: %v", m.Checkers)
	}
	if m.Models[0] != "alpha" || m.Models[1] != "beta" {
		t.Errorf("models not sorted: %v", m.Models)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	m := Build(nil)
	text := FormatText(m)
	if !strings.Contains(text, "No checker/model data") {
		t.Error("empty matrix should say no data")
	}
}

func TestFormatTextTable(t *testing.T) {
	findings := []model.Finding{
		{Lab: "sqli", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevHigh},
		{Lab: "xss", Model: model.ModelRef{Model: "gpt-4"}, Severity: model.SevLow},
	}
	m := Build(findings)
	text := FormatText(m)
	if !strings.Contains(text, "sqli") || !strings.Contains(text, "xss") {
		t.Errorf("table missing checker names:\n%s", text)
	}
	if !strings.Contains(text, "gpt-4") {
		t.Errorf("table missing model name:\n%s", text)
	}
	if !strings.Contains(text, "Checkers: 2") {
		t.Errorf("missing summary:\n%s", text)
	}
}
