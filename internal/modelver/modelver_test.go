package modelver

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(runID, mdl string) model.Finding {
	return model.Finding{
		ID:        "f-" + runID + "-" + mdl,
		RunID:     runID,
		Class:     model.ClassSQLi,
		Severity:  model.SevHigh,
		State:     model.StateValidated,
		Summary:   "test finding",
		Model:     model.ModelRef{Model: mdl, Provider: "test"},
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.TotalRuns != 0 || r.TotalChanges != 0 {
		t.Fatal("expected zero report")
	}
}

func TestSingleRun(t *testing.T) {
	findings := []model.Finding{mkFinding("run-1", "gpt-4-0125")}
	r := Analyze(findings)
	if r.TotalRuns != 1 {
		t.Fatalf("expected 1 run, got %d", r.TotalRuns)
	}
	if len(r.Timeline) != 1 {
		t.Fatalf("expected 1 timeline entry, got %d", len(r.Timeline))
	}
	if r.Timeline[0].Version != "0125" {
		t.Fatalf("expected version 0125, got %q", r.Timeline[0].Version)
	}
}

func TestMultipleRuns(t *testing.T) {
	findings := []model.Finding{
		mkFinding("run-1", "gpt-4-0125"),
		mkFinding("run-2", "gpt-4-0125"),
		mkFinding("run-3", "gpt-4-0125"),
	}
	r := Analyze(findings)
	if r.TotalRuns != 3 {
		t.Fatalf("expected 3 runs, got %d", r.TotalRuns)
	}
	if r.TotalChanges != 0 {
		t.Fatalf("expected 0 changes, got %d", r.TotalChanges)
	}
}

func TestVersionChange(t *testing.T) {
	findings := []model.Finding{
		mkFinding("run-1", "gpt-4-0125"),
		mkFinding("run-2", "gpt-4-0613"),
	}
	r := Analyze(findings)
	if r.TotalChanges != 1 {
		t.Fatalf("expected 1 change, got %d", r.TotalChanges)
	}
	if r.Changes[0].Direction != "upgrade" {
		t.Fatalf("expected upgrade, got %q", r.Changes[0].Direction)
	}
}

func TestVersionDowngrade(t *testing.T) {
	findings := []model.Finding{
		mkFinding("run-1", "gpt-4-0613"),
		mkFinding("run-2", "gpt-4-0125"),
	}
	r := Analyze(findings)
	if r.TotalChanges != 1 {
		t.Fatalf("expected 1 change, got %d", r.TotalChanges)
	}
	if r.Changes[0].Direction != "downgrade" {
		t.Fatalf("expected downgrade, got %q", r.Changes[0].Direction)
	}
}

func TestUnrecordedVersion(t *testing.T) {
	findings := []model.Finding{
		mkFinding("run-1", "custom-model"),
	}
	r := Analyze(findings)
	if r.Unrecorded != 1 {
		t.Fatalf("expected 1 unrecorded, got %d", r.Unrecorded)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("run-1", "gpt-4-0125"),
		mkFinding("run-1", "claude-3-sonnet"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestModelVersionsSeen(t *testing.T) {
	findings := []model.Finding{
		mkFinding("run-1", "gpt-4-0125"),
		mkFinding("run-2", "gpt-4-0613"),
		mkFinding("run-3", "gpt-4-0125"),
	}
	r := Analyze(findings)
	found := false
	for _, ms := range r.Models {
		if ms.Model == "gpt-4" {
			if len(ms.VersionsSeen) != 2 {
				t.Fatalf("expected 2 versions, got %d", len(ms.VersionsSeen))
			}
			found = true
		}
	}
	if !found {
		t.Fatal("model gpt-4 not found")
	}
}

func TestCurrentVersion(t *testing.T) {
	findings := []model.Finding{
		mkFinding("run-1", "gpt-4-0125"),
		mkFinding("run-2", "gpt-4-0613"),
	}
	r := Analyze(findings)
	for _, ms := range r.Models {
		if ms.Model == "gpt-4" {
			if ms.CurrentVersion != "0613" {
				t.Fatalf("expected current 0613, got %q", ms.CurrentVersion)
			}
		}
	}
}

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"gpt-4-0125", "0125"},
		{"claude-3-sonnet", "3-sonnet"},
		{"custom-model", ""},
		{"model-1.5", "1.5"},
		{"", ""},
	}
	for _, tc := range tests {
		got := extractVersion(tc.input)
		if got != tc.want {
			t.Errorf("extractVersion(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestModelBase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"gpt-4-0125", "gpt-4"},
		{"claude-3-sonnet", "claude"},
		{"custom-model", "custom-model"},
	}
	for _, tc := range tests {
		got := modelBase(tc.input)
		if got != tc.want {
			t.Errorf("modelBase(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestPerformanceDelta(t *testing.T) {
	f1 := mkFinding("run-1", "gpt-4-0125")
	f1.Severity = model.SevLow
	f2 := mkFinding("run-2", "gpt-4-0613")
	f2.Severity = model.SevHigh
	findings := []model.Finding{f1, f2}
	r := Analyze(findings)
	if len(r.Performance) == 0 {
		t.Fatal("expected performance deltas")
	}
	if !r.Performance[0].Improved {
		t.Fatal("expected improvement")
	}
}

func TestDedupModelsInRun(t *testing.T) {
	findings := []model.Finding{
		mkFinding("run-1", "gpt-4-0125"),
		mkFinding("run-1", "gpt-4-0125"),
	}
	r := Analyze(findings)
	if len(r.Timeline) != 1 {
		t.Fatalf("expected 1 timeline entry (deduped), got %d", len(r.Timeline))
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("run-1", "gpt-4-0125"),
		mkFinding("run-2", "gpt-4-0613"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Model Version Analysis") {
		t.Fatal("missing header")
	}
	if !strings.Contains(text, "upgrade") {
		t.Fatal("missing upgrade marker")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("run-1", "gpt-4-0125"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.TotalRuns != 1 {
		t.Fatalf("expected 1 run, got %d", parsed.TotalRuns)
	}
}

func TestEmptyModel(t *testing.T) {
	f := model.Finding{
		ID:    "f1",
		RunID: "run-1",
		Model: model.ModelRef{},
	}
	r := Analyze([]model.Finding{f})
	if len(r.Timeline) != 0 {
		t.Fatal("expected no timeline for empty model")
	}
}

func TestChangeCount(t *testing.T) {
	findings := []model.Finding{
		mkFinding("run-1", "gpt-4-0125"),
		mkFinding("run-2", "gpt-4-0613"),
		mkFinding("run-3", "gpt-4-0125"),
	}
	r := Analyze(findings)
	for _, ms := range r.Models {
		if ms.Model == "gpt-4" {
			if ms.ChangeCount != 2 {
				t.Fatalf("expected 2 changes, got %d", ms.ChangeCount)
			}
		}
	}
}
