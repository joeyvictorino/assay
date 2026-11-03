package sourceattrib

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, lab string, evidence int, toolCalls int, reconcileChecked, reconcileMatch bool) model.Finding {
	f := model.Finding{
		Lab:   lab,
		Model: model.ModelRef{Model: modelName},
		Reconcile: model.ReconcileStatus{
			Checked: reconcileChecked,
			Match:   reconcileMatch,
		},
	}
	for i := 0; i < evidence; i++ {
		f.Evidence = append(f.Evidence, model.Evidence{Kind: "http-exchange"})
	}
	for i := 0; i < toolCalls; i++ {
		f.ToolCallIDs = append(f.ToolCallIDs, "tc-"+lab)
	}
	return f
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestFullAttribution(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", 2, 1, true, true),
	}
	r := Analyze(findings)
	if r.Models[0].AttributionScore < 0.9 {
		t.Fatalf("expected high score, got %.2f", r.Models[0].AttributionScore)
	}
}

func TestNoEvidence(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", 0, 0, false, false),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.WithoutEvidence != 1 {
		t.Fatal("expected 1 without evidence")
	}
	if r.Models[0].AttributionScore > 0.1 {
		t.Fatalf("expected low score, got %.2f", r.Models[0].AttributionScore)
	}
}

func TestEvidenceRatio(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", 1, 0, false, false),
		mkFinding("gpt-4", "lab2", 0, 0, false, false),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.EvidenceRatio != 0.5 {
		t.Fatalf("expected evidence ratio 0.5, got %.2f", r.Models[0].Detail.EvidenceRatio)
	}
}

func TestToolCallRatio(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", 0, 1, false, false),
		mkFinding("gpt-4", "lab2", 0, 0, false, false),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.ToolCallRatio != 0.5 {
		t.Fatalf("expected tool call ratio 0.5, got %.2f", r.Models[0].Detail.ToolCallRatio)
	}
}

func TestReconcileRatio(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", 1, 1, true, true),
		mkFinding("gpt-4", "lab2", 1, 1, true, false),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.ReconcileRatio != 0.5 {
		t.Fatalf("expected reconcile ratio 0.5, got %.2f", r.Models[0].Detail.ReconcileRatio)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", 2, 1, true, true),
		mkFinding("claude", "lab1", 0, 0, false, false),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestAttributor != "gpt-4" {
		t.Fatalf("expected gpt-4 best, got %s", r.BestAttributor)
	}
}

func TestGrades(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{0.96, "A"},
		{0.88, "B"},
		{0.75, "C"},
		{0.55, "D"},
		{0.30, "F"},
	}
	for _, tt := range tests {
		got := grade(tt.score)
		if got != tt.want {
			t.Errorf("grade(%.2f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestFormatText(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "Source Attribution") {
		t.Fatal("expected header")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze(nil)
	j := FormatJSON(r)
	if !strings.Contains(j, "models") {
		t.Fatal("expected models key")
	}
}

func TestLabCount(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "lab1", 1, 1, true, true),
		mkFinding("gpt-4", "lab2", 1, 1, true, true),
		mkFinding("gpt-4", "lab3", 1, 1, true, true),
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 3 {
		t.Fatalf("expected 3 labs, got %d", r.Models[0].LabCount)
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab1", 2, 1, true, true),
		mkFinding("m2", "lab1", 2, 1, true, true),
	}
	r := Analyze(findings)
	if r.MeanScore < 0.5 {
		t.Fatalf("expected mean score >= 0.5, got %.2f", r.MeanScore)
	}
}
