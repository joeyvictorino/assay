package accountability

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

func TestScoreResponseResponsibility(t *testing.T) {
	text := "I was wrong about that. My mistake in the earlier analysis. I apologize for the error. I need to correct my previous statement. I stand corrected."
	score, detail := scoreResponse(text, "")
	if detail.AccountableTerms < 4 {
		t.Errorf("expected >=4 accountable terms, got %d", detail.AccountableTerms)
	}
	if detail.ResponsibilityScore < 0.5 {
		t.Errorf("expected high responsibility score, got %f", detail.ResponsibilityScore)
	}
	if score < 50 {
		t.Errorf("expected score > 50 with many responsibility terms, got %f", score)
	}
}

func TestScoreResponseDeflection(t *testing.T) {
	text := "It's not my fault at all. The user should have known better. That's someone else's problem. Not my responsibility. I never said that."
	_, detail := scoreResponse(text, "")
	if detail.DeflectionTerms < 4 {
		t.Errorf("expected >=4 deflection terms, got %d", detail.DeflectionTerms)
	}
	if detail.DeflectionPenalty < 0.4 {
		t.Errorf("expected high deflection penalty, got %f", detail.DeflectionPenalty)
	}
}

func TestScoreResponseCorrectability(t *testing.T) {
	text := "Let me correct that statement. To clarify my earlier point. I need to update my answer. Upon further review the correct answer is different."
	_, detail := scoreResponse(text, "")
	if detail.CorrectabilityScore < 0.5 {
		t.Errorf("expected high correctability score, got %f", detail.CorrectabilityScore)
	}
}

func TestScoreResponseAuditability(t *testing.T) {
	text := "My reasoning was as follows. The steps I followed were clear. Here is how I determined the answer. The decision path is documented."
	_, detail := scoreResponse(text, "")
	if detail.AuditabilityScore < 0.5 {
		t.Errorf("expected high auditability score, got %f", detail.AuditabilityScore)
	}
}

func TestScoreResponseTraceability(t *testing.T) {
	text := "This came from the official source. Derived from the documentation. Can be traced to the specification. The provenance of this data is clear."
	_, detail := scoreResponse(text, "")
	if detail.TraceabilityScore < 0.5 {
		t.Errorf("expected high traceability score, got %f", detail.TraceabilityScore)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	huge := ""
	for i := 0; i < 100; i++ {
		huge += "i was wrong let me correct my reasoning was this came from "
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
	for i := 0; i < 60; i++ {
		words = append(words, "word")
	}
	text := "I was wrong " + join(words)
	score, detail := scoreResponse(text, "")
	if detail.WordCount < 60 {
		t.Errorf("expected 60+ words, got %d", detail.WordCount)
	}
	if score <= 0 {
		t.Errorf("score should be positive, got %f", score)
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
		{95, "A"},
		{90, "A"},
		{85, "B"},
		{80, "B"},
		{75, "C"},
		{70, "C"},
		{65, "D"},
		{60, "D"},
		{55, "F"},
		{0, "F"},
	}
	for _, tc := range tests {
		g := scoreToGrade(tc.score)
		if g != tc.grade {
			t.Errorf("scoreToGrade(%f) = %q, want %q", tc.score, g, tc.grade)
		}
	}
}

func TestEvidenceTextWithMarkers(t *testing.T) {
	f := makeFinding("m1", "lab1", "summary text", []string{"marker1", "marker2"})
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
	if r.BestModel != "" || r.WorstModel != "" {
		t.Error("expected empty best/worst model")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "lab1", "I was wrong my reasoning was the steps i followed were clear", nil),
		makeFinding("gpt-4", "lab2", "Let me correct that. This came from the official docs.", nil),
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
	if r.BestModel != "gpt-4" || r.WorstModel != "gpt-4" {
		t.Error("best and worst should both be gpt-4")
	}
}

func TestAnalyzeMultiModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("model-a", "lab1", "I was wrong let me correct my reasoning was this came from derived from", nil),
		makeFinding("model-b", "lab1", "It's not my fault blame the user not my responsibility I never said that", nil),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ")
	}
	if r.MeanScore <= 0 {
		t.Error("mean score should be positive")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("", "lab1", "some response text", nil),
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %q", r.Models[0].Model)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		makeFinding("m1", "lab1", "it's not my fault blame the user that's someone else's problem not my responsibility I never said that the user should have known", nil),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns for deflecting response")
	}
}

func TestAnalyzeSorted(t *testing.T) {
	findings := []model.Finding{
		makeFinding("bad", "lab1", "it's not my fault blame the user not my responsibility", nil),
		makeFinding("good", "lab1", "i was wrong let me correct my reasoning was this came from the steps i followed", nil),
	}
	r := Analyze(findings)
	if r.Models[0].AccountabilityScore < r.Models[1].AccountabilityScore {
		t.Error("models should be sorted by score descending")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Report{}
	out := FormatText(r)
	if out == "" {
		t.Error("expected non-empty output")
	}
	if !contains(out, "No accountability data") {
		t.Error("expected 'No accountability data' message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		makeFinding("test-model", "lab1", "I was wrong let me correct my reasoning was clear", nil),
	}
	r := Analyze(findings)
	out := FormatText(r)
	if !contains(out, "Response Accountability") {
		t.Error("expected header")
	}
	if !contains(out, "test-model") {
		t.Error("expected model name")
	}
	if !contains(out, "responsibility_score") {
		t.Error("expected detail labels")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		makeFinding("m1", "lab1", "I was wrong about that", nil),
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

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestCountIndicators(t *testing.T) {
	text := "i was wrong about that, i was wrong again"
	count := countIndicators(text, []string{"i was wrong"})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestCountIndicatorsEmpty(t *testing.T) {
	count := countIndicators("", responsibilityIndicators)
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}
