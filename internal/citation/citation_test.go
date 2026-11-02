package citation

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(m, evidence string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: m},
		Summary: evidence,
		Evidence: []model.Evidence{{Kind: "text", Marker: evidence}},
	}
}

func TestClassifyPresence(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "According to the official documentation"),
		mkFinding("m", "Plain response"),
	}
	ds := classifyDimension(DimPresence, findings)
	if ds.Count != 1 {
		t.Errorf("expected 1 presence, got %d", ds.Count)
	}
}

func TestClassifyFormat(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "Smith et al. published in IEEE 2023"),
		mkFinding("m", "Plain response"),
	}
	ds := classifyDimension(DimFormat, findings)
	if ds.Count != 1 {
		t.Errorf("expected 1 format, got %d", ds.Count)
	}
}

func TestClassifySpecificity(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "See section 3.2 for details"),
		mkFinding("m", "Plain answer"),
	}
	ds := classifyDimension(DimSpecificity, findings)
	if ds.Count != 1 {
		t.Errorf("expected 1 specificity, got %d", ds.Count)
	}
}

func TestClassifyFabrication(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "I believe this comes from some paper"),
		mkFinding("m", "Clearly stated in RFC 7231"),
	}
	ds := classifyDimension(DimFabrication, findings)
	if ds.Count != 1 {
		t.Errorf("expected 1 fabrication, got %d", ds.Count)
	}
}

func TestClassifyGeneral(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "See https://example.com/docs for more info"),
		mkFinding("m", "No URL here"),
	}
	ds := classifyDimension(DimGeneral, findings)
	if ds.Count != 1 {
		t.Errorf("expected 1 general (URL), got %d", ds.Count)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected no models")
	}
}

func TestAnalyzeSingle(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "According to the docs"),
		mkFinding("gpt-4", "See section 2"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Errorf("expected 1 model, got %d", len(r.Models))
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "According to the source"),
		mkFinding("claude", "I believe maybe"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(r.Models))
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		rate float64
		want string
	}{
		{0.9, "A"},
		{0.7, "B"},
		{0.5, "C"},
		{0.3, "D"},
		{0.1, "F"},
	}
	for _, tt := range tests {
		got := grade(tt.rate)
		if got != tt.want {
			t.Errorf("grade(%.1f) = %s, want %s", tt.rate, got, tt.want)
		}
	}
}

func TestBestWorst(t *testing.T) {
	findings := []model.Finding{
		mkFinding("good", "According to [1] Smith et al."),
		mkFinding("good", "See section 3, doi:10.1234/test"),
		mkFinding("bad", "I think maybe this is right"),
	}
	r := Analyze(findings)
	if r.Best == "" || r.Worst == "" {
		t.Error("expected best and worst")
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "According to the source"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Citation") {
		t.Error("missing header")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("missing model name")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("gpt-4", "According to the documentation"),
	}
	r := Analyze(findings)
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(FormatJSON(r)), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestScoreRange(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "According to source [1]"),
	}
	r := Analyze(findings)
	idx := r.Models[0].CitationIdx
	if idx < 0 || idx > 1 {
		t.Errorf("citation index out of range: %.2f", idx)
	}
}

func TestCitationIndex(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m", "According to Smith et al. in section 3, see https://example.com doi:10.1234"),
		mkFinding("m", "As stated in [1] on page 42 per the documentation"),
	}
	r := Analyze(findings)
	if r.Models[0].CitationIdx == 0 {
		t.Error("expected non-zero citation index")
	}
}
