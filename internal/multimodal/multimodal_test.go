package multimodal

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func findings(summaries ...string) []model.Finding {
	var fs []model.Finding
	for _, s := range summaries {
		fs = append(fs, model.Finding{Summary: s})
	}
	return fs
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if r.TotalFindings != 0 {
		t.Errorf("expected 0 findings, got %d", r.TotalFindings)
	}
}

func TestDetectModalities(t *testing.T) {
	mods := detectModalities("the screenshot shows a code block with def main()")
	seen := map[string]bool{}
	for _, m := range mods {
		seen[m] = true
	}
	if !seen["text"] {
		t.Error("expected text modality")
	}
	if !seen["image"] {
		t.Error("expected image modality from 'screenshot'")
	}
	if !seen["code"] {
		t.Error("expected code modality from 'def '")
	}
}

func TestMultiModalCount(t *testing.T) {
	fs := findings(
		"the image shows a vulnerability in the code snippet",
		"plain text finding with no other modality",
		"the diagram illustrates the attack path via screenshot",
	)
	r := Analyze(fs)
	if r.MultiModalCount != 2 {
		t.Errorf("expected 2 multi-modal, got %d", r.MultiModalCount)
	}
	if r.SingleModalCount != 1 {
		t.Errorf("expected 1 single-modal, got %d", r.SingleModalCount)
	}
}

func TestCrossModalConsistencyPenalty(t *testing.T) {
	fs := findings(
		"the image shows X however the image shows something different",
		"text says Y but contrary to the text the data disagrees",
	)
	r := Analyze(fs)
	var consist DimensionScore
	for _, ds := range r.DimensionScores {
		if ds.Dimension == DimCrossModalConsist {
			consist = ds
			break
		}
	}
	if consist.Score >= 1.0 {
		t.Errorf("expected consistency score < 1.0 for inconsistent findings, got %.2f", consist.Score)
	}
}

func TestPerfectConsistency(t *testing.T) {
	fs := findings(
		"the code handles errors properly",
		"the configuration is correct",
	)
	r := Analyze(fs)
	var consist DimensionScore
	for _, ds := range r.DimensionScores {
		if ds.Dimension == DimCrossModalConsist {
			consist = ds
			break
		}
	}
	if consist.Score != 1.0 {
		t.Errorf("expected consistency score 1.0 for consistent findings, got %.2f", consist.Score)
	}
}

func TestFormatText(t *testing.T) {
	fs := findings("some finding about an image in code")
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Multi-Modal") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "Overall Score") {
		t.Error("expected overall score in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	fs := findings("some finding")
	r := Analyze(fs)
	j := FormatJSON(r)
	if !strings.Contains(j, "overall_score") {
		t.Error("expected overall_score in JSON output")
	}
}

func TestOverallScoreRange(t *testing.T) {
	fs := findings(
		"a plain text finding",
		"an image based finding",
		"a code snippet finding",
	)
	r := Analyze(fs)
	if r.OverallScore < 0.0 || r.OverallScore > 1.0 {
		t.Errorf("overall score out of range: %.2f", r.OverallScore)
	}
}
