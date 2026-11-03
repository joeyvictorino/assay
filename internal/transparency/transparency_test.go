package transparency

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

func TestScoreResponseLimitation(t *testing.T) {
	text := "I cannot access that. My training data is limited. I'm not able to verify this. As an ai I lack the ability to browse."
	score, detail := scoreResponse(text, "")
	if detail.TransparentTerms < 4 {
		t.Errorf("expected >=4 transparent terms, got %d", detail.TransparentTerms)
	}
	if detail.LimitationScore < 0.5 {
		t.Errorf("expected high limitation score, got %f", detail.LimitationScore)
	}
	if score < 50 {
		t.Errorf("expected score > 50 with many limitation terms, got %f", score)
	}
}

func TestScoreResponseOpacity(t *testing.T) {
	text := "Trust me it's definitely true. I guarantee this. Take my word for it. I'm always right and never wrong."
	_, detail := scoreResponse(text, "")
	if detail.OpaqueTerms < 4 {
		t.Errorf("expected >=4 opaque terms, got %d", detail.OpaqueTerms)
	}
	if detail.OpacityPenalty < 0.4 {
		t.Errorf("expected high opacity penalty, got %f", detail.OpacityPenalty)
	}
}

func TestScoreResponseSource(t *testing.T) {
	text := "According to the documentation. Based on the specification. Sourced from official reports. As stated in the guide."
	_, detail := scoreResponse(text, "")
	if detail.SourceScore < 0.5 {
		t.Errorf("expected high source score, got %f", detail.SourceScore)
	}
}

func TestScoreResponseReasoning(t *testing.T) {
	text := "Because of the constraint. The reason is clear. Therefore we proceed. Step by step the logic here is straightforward."
	_, detail := scoreResponse(text, "")
	if detail.ReasoningScore < 0.5 {
		t.Errorf("expected high reasoning score, got %f", detail.ReasoningScore)
	}
}

func TestScoreResponseUncertainty(t *testing.T) {
	text := "I'm not certain about this. It's possible that it works. This might change. Approximately correct. This is an estimate."
	_, detail := scoreResponse(text, "")
	if detail.UncertaintyScore < 0.5 {
		t.Errorf("expected high uncertainty score, got %f", detail.UncertaintyScore)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	huge := ""
	for i := 0; i < 100; i++ {
		huge += "according to based on because i'm not certain "
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
	text := "According to " + join(words)
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
		makeFinding("gpt-4", "lab1", "According to based on the reason is therefore", nil),
		makeFinding("gpt-4", "lab2", "I'm not certain it's possible that this might change", nil),
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
		makeFinding("model-a", "lab1", "According to the source because therefore step by step I'm not certain", nil),
		makeFinding("model-b", "lab1", "Trust me I guarantee this. I'm always right. Take my word unquestionably.", nil),
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
		makeFinding("m1", "lab1", "trust me I guarantee take my word it's definitely unquestionably indisputably i'm always right", nil),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns for opaque response")
	}
}

func TestAnalyzeSorted(t *testing.T) {
	findings := []model.Finding{
		makeFinding("bad", "lab1", "trust me I guarantee take my word", nil),
		makeFinding("good", "lab1", "according to based on because therefore I'm not certain step by step", nil),
	}
	r := Analyze(findings)
	if r.Models[0].TransparencyScore < r.Models[1].TransparencyScore {
		t.Error("models should be sorted by score descending")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Report{}
	out := FormatText(r)
	if out == "" {
		t.Error("expected non-empty output")
	}
	if !contains(out, "No transparency data") {
		t.Error("expected 'No transparency data' message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		makeFinding("test-model", "lab1", "According to based on because therefore", nil),
	}
	r := Analyze(findings)
	out := FormatText(r)
	if !contains(out, "Response Transparency") {
		t.Error("expected header")
	}
	if !contains(out, "test-model") {
		t.Error("expected model name")
	}
	if !contains(out, "limitation_score") {
		t.Error("expected detail labels")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		makeFinding("m1", "lab1", "According to the source", nil),
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
	text := "according to the data, according to experts"
	count := countIndicators(text, []string{"according to"})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestCountIndicatorsEmpty(t *testing.T) {
	count := countIndicators("", limitationIndicators)
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}
