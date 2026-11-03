package normalize

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(id, class, method, path, sev string) model.Finding {
	return model.Finding{
		ID:       id,
		Class:    model.Class(class),
		Severity: model.Severity(sev),
		Location: model.Location{Method: method, PathTemplate: path},
		Summary:  "test finding " + id,
		Model:    model.ModelRef{Provider: "openai", Model: "gpt-4"},
		Lab:      "lab1",
	}
}

func TestDeduplication(t *testing.T) {
	fs := []model.Finding{
		finding("f1", "sqli", "GET", "/api/users", "high"),
		finding("f2", "sqli", "GET", "/api/users", "high"),
		finding("f3", "xss-reflected", "POST", "/form", "medium"),
	}
	r := Analyze(fs)
	if r.TotalInput != 3 {
		t.Errorf("TotalInput = %d, want 3", r.TotalInput)
	}
	if r.TotalOutput != 2 {
		t.Errorf("TotalOutput = %d, want 2", r.TotalOutput)
	}
	if r.DuplicatesFound != 1 {
		t.Errorf("DuplicatesFound = %d, want 1", r.DuplicatesFound)
	}
}

func TestSeverityNormalization(t *testing.T) {
	tests := []struct {
		input string
		want  model.Severity
	}{
		{"critical", model.SevCritical},
		{"crit", model.SevCritical},
		{"high", model.SevHigh},
		{"h", model.SevHigh},
		{"medium", model.SevMedium},
		{"med", model.SevMedium},
		{"low", model.SevLow},
		{"info", model.SevInfo},
		{"informational", model.SevInfo},
		{"note", model.SevInfo},
		{"unknown", model.SevInfo},
	}
	for _, tt := range tests {
		got := normalizeSeverity(model.Severity(tt.input))
		if got != tt.want {
			t.Errorf("normalizeSeverity(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMergeCount(t *testing.T) {
	fs := []model.Finding{
		finding("f1", "sqli", "GET", "/a", "high"),
		finding("f2", "sqli", "GET", "/a", "high"),
		finding("f3", "sqli", "GET", "/a", "critical"),
	}
	r := Analyze(fs)
	if r.TotalOutput != 1 {
		t.Fatalf("expected 1 merged finding, got %d", r.TotalOutput)
	}
	if r.Findings[0].MergeCount != 3 {
		t.Errorf("MergeCount = %d, want 3", r.Findings[0].MergeCount)
	}
	if len(r.Findings[0].OriginalIDs) != 3 {
		t.Errorf("OriginalIDs count = %d, want 3", len(r.Findings[0].OriginalIDs))
	}
}

func TestSortBySeverity(t *testing.T) {
	fs := []model.Finding{
		finding("f1", "sqli", "GET", "/a", "low"),
		finding("f2", "xss-reflected", "POST", "/b", "critical"),
		finding("f3", "idor", "GET", "/c", "medium"),
	}
	r := Analyze(fs)
	if r.Findings[0].Severity != model.SevCritical {
		t.Errorf("first finding severity = %q, want critical", r.Findings[0].Severity)
	}
}

func TestCanonicalID(t *testing.T) {
	fs := []model.Finding{
		finding("f1", "sqli", "GET", "/a", "high"),
	}
	r := Analyze(fs)
	if !strings.HasPrefix(r.Findings[0].CanonicalID, "ASSAY-SQLI-") {
		t.Errorf("CanonicalID = %q, want ASSAY-SQLI- prefix", r.Findings[0].CanonicalID)
	}
}

func TestEmptyInput(t *testing.T) {
	r := Analyze(nil)
	if r.TotalInput != 0 || r.TotalOutput != 0 {
		t.Error("expected zeros for empty input")
	}
}

func TestSeverityCounts(t *testing.T) {
	fs := []model.Finding{
		finding("f1", "sqli", "GET", "/a", "high"),
		finding("f2", "xss-reflected", "POST", "/b", "high"),
		finding("f3", "idor", "GET", "/c", "low"),
	}
	r := Analyze(fs)
	if r.SeverityCounts["high"] != 2 {
		t.Errorf("high count = %d, want 2", r.SeverityCounts["high"])
	}
	if r.SeverityCounts["low"] != 1 {
		t.Errorf("low count = %d, want 1", r.SeverityCounts["low"])
	}
}

func TestFormatText(t *testing.T) {
	fs := []model.Finding{
		finding("f1", "sqli", "GET", "/a", "high"),
	}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Findings Normalization Report") {
		t.Error("missing title")
	}
	if !strings.Contains(text, "ASSAY-SQLI-") {
		t.Error("missing canonical ID")
	}
}

func TestFormatJSON(t *testing.T) {
	fs := []model.Finding{
		finding("f1", "sqli", "GET", "/a", "high"),
	}
	r := Analyze(fs)
	j := FormatJSON(r)
	if !strings.Contains(j, "canonical_id") {
		t.Error("missing canonical_id in JSON")
	}
}
