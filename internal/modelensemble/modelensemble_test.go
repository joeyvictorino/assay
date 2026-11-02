package modelensemble

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(id, dedupKey, mdl, state string) model.Finding {
	return model.Finding{
		ID:       id,
		DedupKey: dedupKey,
		Model:    model.ModelRef{Provider: "test", Model: mdl},
		State:    model.FindingState(state),
		Summary:  "test finding " + id,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.Summary.TotalFindings != 0 {
		t.Fatalf("expected 0, got %d", r.Summary.TotalFindings)
	}
}

func TestAnalyzeUnanimous(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "dup1", "gpt-4", "validated"),
		makeFinding("f2", "dup1", "claude-3", "validated"),
		makeFinding("f3", "dup1", "gemini", "validated"),
	}
	r := Analyze(findings)
	if r.Summary.TotalFindings != 1 {
		t.Fatalf("expected 1 finding group, got %d", r.Summary.TotalFindings)
	}
	if r.Summary.UnanimousCount != 1 {
		t.Fatalf("expected 1 unanimous, got %d", r.Summary.UnanimousCount)
	}
	if r.Results[0].Agreement != 1.0 {
		t.Fatalf("expected 100%% agreement, got %f", r.Results[0].Agreement)
	}
}

func TestAnalyzeMajority(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "dup1", "gpt-4", "validated"),
		makeFinding("f2", "dup1", "claude-3", "validated"),
		makeFinding("f3", "dup1", "gemini", "refuted"),
	}
	r := Analyze(findings)
	if r.Summary.MajorityCount != 1 {
		t.Fatalf("expected 1 majority, got %d", r.Summary.MajorityCount)
	}
}

func TestAnalyzeDisputed(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "dup1", "gpt-4", "validated"),
		makeFinding("f2", "dup1", "claude-3", "refuted"),
		makeFinding("f3", "dup1", "gemini", "refuted"),
	}
	r := Analyze(findings)
	if r.Summary.DisputedCount != 1 {
		t.Fatalf("expected 1 disputed, got %d", r.Summary.DisputedCount)
	}
}

func TestAnalyzeMultipleGroups(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "dup1", "gpt-4", "validated"),
		makeFinding("f2", "dup1", "claude-3", "validated"),
		makeFinding("f3", "dup2", "gpt-4", "theorized"),
		makeFinding("f4", "dup2", "claude-3", "refuted"),
	}
	r := Analyze(findings)
	if r.Summary.TotalFindings != 2 {
		t.Fatalf("expected 2 groups, got %d", r.Summary.TotalFindings)
	}
}

func TestAnalyzeNoDedupKey(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "", "gpt-4", "validated"),
		makeFinding("f2", "", "claude-3", "validated"),
	}
	r := Analyze(findings)
	if r.Summary.TotalFindings != 2 {
		t.Fatalf("expected 2 groups (no dedup key), got %d", r.Summary.TotalFindings)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "dup1", "gpt-4", "validated"),
		makeFinding("f2", "dup1", "claude-3", "refuted"),
	}
	r := Analyze(findings)
	out := FormatText(r)
	if !strings.Contains(out, "Multi-Model Ensemble") {
		t.Fatal("expected header")
	}
	if !strings.Contains(out, "MAJORITY") {
		t.Fatal("expected MAJORITY label")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		makeFinding("f1", "dup1", "gpt-4", "validated"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	if !strings.Contains(out, `"total_findings"`) {
		t.Fatal("expected total_findings in JSON")
	}
}
