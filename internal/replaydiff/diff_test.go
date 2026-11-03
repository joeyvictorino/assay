package replaydiff

import (
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestDiffNewFindings(t *testing.T) {
	base := model.RunReport{RunID: "run-1"}
	cur := model.RunReport{RunID: "run-2"}

	baseFindings := []model.Finding{
		{ID: "f1", DedupKey: "k1", Lab: "lab-a", Model: model.ModelRef{Model: "m1"}, Severity: model.SevHigh, Summary: "found vuln"},
	}
	curFindings := []model.Finding{
		{ID: "f1b", DedupKey: "k1", Lab: "lab-a", Model: model.ModelRef{Model: "m1"}, Severity: model.SevHigh, Summary: "found vuln"},
		{ID: "f2", DedupKey: "k2", Lab: "lab-b", Model: model.ModelRef{Model: "m1"}, Severity: model.SevMedium, Summary: "new issue"},
	}

	d := Diff(base, cur, baseFindings, curFindings)
	if d.Summary.NewCount != 1 {
		t.Errorf("new count = %d, want 1", d.Summary.NewCount)
	}
	if d.Summary.ResolvedCount != 0 {
		t.Errorf("resolved count = %d, want 0", d.Summary.ResolvedCount)
	}
	if d.New[0].DedupKey != "k2" {
		t.Errorf("new finding dedup_key = %s, want k2", d.New[0].DedupKey)
	}
}

func TestDiffResolvedFindings(t *testing.T) {
	base := model.RunReport{RunID: "run-1"}
	cur := model.RunReport{RunID: "run-2"}

	baseFindings := []model.Finding{
		{ID: "f1", DedupKey: "k1", Lab: "lab-a", Model: model.ModelRef{Model: "m1"}, Severity: model.SevHigh},
		{ID: "f2", DedupKey: "k2", Lab: "lab-b", Model: model.ModelRef{Model: "m1"}, Severity: model.SevLow},
	}
	curFindings := []model.Finding{
		{ID: "f1b", DedupKey: "k1", Lab: "lab-a", Model: model.ModelRef{Model: "m1"}, Severity: model.SevHigh},
	}

	d := Diff(base, cur, baseFindings, curFindings)
	if d.Summary.ResolvedCount != 1 {
		t.Errorf("resolved count = %d, want 1", d.Summary.ResolvedCount)
	}
	if d.Resolved[0].DedupKey != "k2" {
		t.Errorf("resolved finding dedup_key = %s, want k2", d.Resolved[0].DedupKey)
	}
}

func TestDiffChangedSeverity(t *testing.T) {
	base := model.RunReport{RunID: "run-1"}
	cur := model.RunReport{RunID: "run-2"}

	baseFindings := []model.Finding{
		{ID: "f1", DedupKey: "k1", Lab: "lab-a", Model: model.ModelRef{Model: "m1"}, Severity: model.SevLow},
	}
	curFindings := []model.Finding{
		{ID: "f1b", DedupKey: "k1", Lab: "lab-a", Model: model.ModelRef{Model: "m1"}, Severity: model.SevHigh},
	}

	d := Diff(base, cur, baseFindings, curFindings)
	if d.Summary.ChangedCount != 1 {
		t.Errorf("changed count = %d, want 1", d.Summary.ChangedCount)
	}
	if d.Changed[0].OldScore != 1 || d.Changed[0].NewScore != 3 {
		t.Errorf("changed scores = %.0f→%.0f, want 1→3", d.Changed[0].OldScore, d.Changed[0].NewScore)
	}
}

func TestDiffNoDifferences(t *testing.T) {
	base := model.RunReport{RunID: "a"}
	cur := model.RunReport{RunID: "b"}

	findings := []model.Finding{
		{ID: "f1", DedupKey: "k1", Lab: "lab-a", Model: model.ModelRef{Model: "m1"}, Severity: model.SevHigh},
	}

	d := Diff(base, cur, findings, findings)
	if d.Summary.NewCount != 0 || d.Summary.ResolvedCount != 0 || d.Summary.ChangedCount != 0 {
		t.Errorf("expected no differences, got new=%d resolved=%d changed=%d",
			d.Summary.NewCount, d.Summary.ResolvedCount, d.Summary.ChangedCount)
	}
}

func TestDiffEmptyRuns(t *testing.T) {
	d := Diff(model.RunReport{RunID: "a"}, model.RunReport{RunID: "b"}, nil, nil)
	if d.Summary.NewCount != 0 || d.Summary.ResolvedCount != 0 || d.Summary.ChangedCount != 0 {
		t.Error("empty runs should have no differences")
	}
	if d.BaseRunID != "a" || d.CurRunID != "b" {
		t.Error("run IDs not set correctly")
	}
}

func TestFormatText(t *testing.T) {
	d := DiffResult{
		BaseRunID: "run-1",
		CurRunID:  "run-2",
		New: []model.Finding{
			{Severity: model.SevHigh, Summary: "new vuln", Lab: "lab-a", Model: model.ModelRef{Model: "m1"}},
		},
		Resolved: []model.Finding{
			{Severity: model.SevLow, Summary: "old vuln", Lab: "lab-b", Model: model.ModelRef{Model: "m2"}},
		},
		Summary: DiffSummary{NewCount: 1, ResolvedCount: 1},
	}

	text := FormatText(d)
	if text == "" {
		t.Fatal("empty output")
	}
	for _, want := range []string{"run-1", "run-2", "New Findings", "Resolved Findings", "new vuln", "old vuln"} {
		if !contains(text, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && containsStr(s, sub)
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
