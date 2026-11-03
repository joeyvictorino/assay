package cli

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestAnalyzeImpactEmpty(t *testing.T) {
	rr := model.RunReport{}
	r := analyzeImpact(rr, 0)
	if r.TotalFindings != 0 {
		t.Errorf("expected 0 findings, got %d", r.TotalFindings)
	}
	if r.AggregateScore != 0 {
		t.Errorf("expected 0 score, got %.1f", r.AggregateScore)
	}
}

func TestAnalyzeImpactWithLabs(t *testing.T) {
	rr := model.RunReport{
		Labs: []string{"sqli_basic", "xss_reflected", "auth_bypass_token"},
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Model: "gpt-4"}, Findings: map[model.FindingState]int{"VALIDATED": 2}},
			{Ref: model.ModelRef{Model: "claude"}, Findings: map[model.FindingState]int{"VALIDATED": 1}},
		},
	}
	r := analyzeImpact(rr, 0)
	if r.TotalFindings != 3 {
		t.Errorf("expected 3 labs, got %d", r.TotalFindings)
	}
	if r.AggregateScore == 0 {
		t.Error("expected non-zero aggregate score")
	}
	if len(r.AreaBreakdown) == 0 {
		t.Error("expected area breakdown")
	}
}

func TestAnalyzeImpactThreshold(t *testing.T) {
	rr := model.RunReport{
		Labs:   []string{"info_leak_basic", "rce_critical"},
		Models: []model.ModelResult{{Ref: model.ModelRef{Model: "m1"}, Findings: map[model.FindingState]int{"VALIDATED": 1}}},
	}
	r := analyzeImpact(rr, 100)
	if len(r.TopImpact) != 0 {
		t.Errorf("expected 0 entries above threshold, got %d", len(r.TopImpact))
	}
}

func TestClassifyLabImpact(t *testing.T) {
	areas := classifyLabImpact("sqli_blind")
	if len(areas) != 2 {
		t.Errorf("sqli should have 2 areas, got %d", len(areas))
	}
	areas2 := classifyLabImpact("unknown_lab")
	if len(areas2) != 1 || areas2[0] != "confidentiality" {
		t.Errorf("unknown lab should default to confidentiality, got %v", areas2)
	}
}

func TestLabImpactScore(t *testing.T) {
	score := labImpactScore("rce_shell", 3)
	if score <= 0 {
		t.Error("expected positive score")
	}
	score2 := labImpactScore("info_leak", 1)
	if score2 >= score {
		t.Error("rce should score higher than info_leak")
	}
}

func TestFormatImpactText(t *testing.T) {
	r := impactResult{
		TotalFindings:  5,
		AggregateScore: 25.0,
		AreaBreakdown:  []impactArea{{Name: "confidentiality", Count: 3, Score: 15.0}},
		TopImpact:      []impactEntry{{Lab: "sqli", Areas: []string{"data_integrity"}, Score: 8.0, ModelHits: 2}},
	}
	text := formatImpactText(r)
	if !strings.Contains(text, "Findings Impact Analysis") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "25.0") {
		t.Error("missing aggregate score")
	}
	if !strings.Contains(text, "confidentiality") {
		t.Error("missing area")
	}
}
