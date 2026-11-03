package thoroughness

import (
	"encoding/json"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(mn, lab, summary string, markers []string) model.Finding {
	var ev []model.Evidence
	for _, m := range markers {
		ev = append(ev, model.Evidence{Marker: m})
	}
	return model.Finding{
		Model:    model.ModelRef{Model: mn},
		Lab:      lab,
		Summary:  summary,
		Evidence: ev,
	}
}

func TestScoreResponseBaseline(t *testing.T) {
	score, detail := scoreResponse("hello world", "")
	if score < 0 || score > 100 {
		t.Fatalf("score out of range: %f", score)
	}
	if detail.WordCount < 2 {
		t.Errorf("expected >=2 words, got %d", detail.WordCount)
	}
}

func TestScoreResponseCoverage(t *testing.T) {
	text := "Comprehensively thoroughly exhaustively covering all across all each aspect every dimension all factors"
	score, detail := scoreResponse(text, "")
	if detail.CoverageTerms < 5 {
		t.Errorf("expected >=5 coverage terms, got %d", detail.CoverageTerms)
	}
	if detail.CoverageScore < 0.5 {
		t.Errorf("expected high coverage score, got %f", detail.CoverageScore)
	}
	if score < 40 {
		t.Errorf("expected score > 40, got %f", score)
	}
}

func TestScoreResponseDetail(t *testing.T) {
	text := "Specifically in detail precisely the exact granular fine-grained step by step broken down"
	_, detail := scoreResponse(text, "")
	if detail.DetailScore < 0.5 {
		t.Errorf("expected high detail score, got %f", detail.DetailScore)
	}
}

func TestScoreResponseMethod(t *testing.T) {
	text := "Methodology systematic framework approach procedure protocol technique strategy"
	_, detail := scoreResponse(text, "")
	if detail.MethodScore < 0.5 {
		t.Errorf("expected high method score, got %f", detail.MethodScore)
	}
}

func TestScoreResponseVerification(t *testing.T) {
	text := "Verified confirmed validated cross-checked double-checked corroborated tested against"
	_, detail := scoreResponse(text, "")
	if detail.VerificationScore < 0.5 {
		t.Errorf("expected high verification score, got %f", detail.VerificationScore)
	}
}

func TestScoreResponseEdgeCases(t *testing.T) {
	text := "Edge case corner case boundary condition special case exception unusual scenario worst case"
	_, detail := scoreResponse(text, "")
	if detail.EdgeCaseScore < 0.5 {
		t.Errorf("expected high edge case score, got %f", detail.EdgeCaseScore)
	}
}

func TestScoreResponseGaps(t *testing.T) {
	text := "Not covered omitted skipped left out missing incomplete partial superficial cursory"
	_, detail := scoreResponse(text, "")
	if detail.GapTerms < 5 {
		t.Errorf("expected >=5 gap terms, got %d", detail.GapTerms)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	huge := ""
	for i := 0; i < 100; i++ {
		huge += "comprehensively thoroughly verified confirmed edge case "
	}
	score, _ := scoreResponse(huge, "")
	if score > 100 {
		t.Errorf("score exceeded 100: %f", score)
	}
	if score < 0 {
		t.Errorf("score below 0: %f", score)
	}
}

func TestScoreResponseLongText(t *testing.T) {
	var words []string
	for i := 0; i < 90; i++ {
		words = append(words, "word")
	}
	text := "comprehensively " + join(words)
	_, detail := scoreResponse(text, "")
	if detail.WordCount < 90 {
		t.Errorf("expected 90+ words, got %d", detail.WordCount)
	}
}

func join(ss []string) string {
	r := ""
	for i, s := range ss {
		if i > 0 {
			r += " "
		}
		r += s
	}
	return r
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {90, "A"}, {85, "B"}, {80, "B"},
		{75, "C"}, {70, "C"}, {65, "D"}, {60, "D"},
		{55, "F"}, {0, "F"},
	}
	for _, tc := range tests {
		g := scoreToGrade(tc.score)
		if g != tc.grade {
			t.Errorf("scoreToGrade(%f) = %q, want %q", tc.score, g, tc.grade)
		}
	}
}

func TestEvidenceTextWithMarkers(t *testing.T) {
	f := makeFinding("m1", "lab1", "summary", []string{"marker1", "marker2"})
	et := evidenceText(f)
	if et != "marker1 marker2" {
		t.Errorf("expected 'marker1 marker2', got %q", et)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := makeFinding("m1", "lab1", "summary text", nil)
	et := evidenceText(f)
	if et != "summary text" {
		t.Errorf("expected 'summary text', got %q", et)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected no models, got %d", len(r.Models))
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "Comprehensively thoroughly covering all aspects step by step", nil),
		makeFinding("gpt-4", "lab2", "Methodology systematic framework verified confirmed", nil),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("model-a", "lab1", "Comprehensively thoroughly verified confirmed edge case methodology", nil),
		makeFinding("model-b", "lab1", "Not covered omitted skipped left out missing incomplete", nil),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("", "lab1", "some text", nil),
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %q", r.Models[0].Model)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		makeFinding("m1", "lab1", "not covered omitted skipped left out missing incomplete partial superficial", nil),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns for low-coverage response")
	}
}

func TestAnalyzeSorted(t *testing.T) {
	findings := []model.Finding{
		makeFinding("bad", "lab1", "not covered omitted skipped", nil),
		makeFinding("good", "lab1", "comprehensively thoroughly verified confirmed methodology edge case", nil),
	}
	r := Analyze(findings)
	if r.Models[0].ThoroughnessScore < r.Models[1].ThoroughnessScore {
		t.Error("models should be sorted by score descending")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Report{}
	out := FormatText(r)
	if out == "" {
		t.Error("expected non-empty output")
	}
	if !contains(out, "No thoroughness data") {
		t.Error("expected 'No thoroughness data' message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		makeFinding("test-model", "lab1", "Comprehensively thoroughly step by step", nil),
	}
	r := Analyze(findings)
	out := FormatText(r)
	if !contains(out, "Response Thoroughness") {
		t.Error("expected header")
	}
	if !contains(out, "test-model") {
		t.Error("expected model name")
	}
	if !contains(out, "coverage_score") {
		t.Error("expected detail labels")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		makeFinding("m1", "lab1", "comprehensively verified", nil),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("expected 1 model in JSON, got %d", len(parsed.Models))
	}
}

func TestFormatJSONRoundtrip(t *testing.T) {
	r := Report{}
	out := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.BestModel != "" {
		t.Error("expected empty best model")
	}
}

func TestCountIndicators(t *testing.T) {
	text := "comprehensively and again comprehensively"
	count := countIndicators(text, []string{"comprehensively"})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestCountIndicatorsEmpty(t *testing.T) {
	count := countIndicators("", coverageIndicators)
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
