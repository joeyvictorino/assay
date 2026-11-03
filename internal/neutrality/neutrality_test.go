package neutrality

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

func TestScoreResponseObjective(t *testing.T) {
	text := "According to research shows studies indicate data suggests evidence points analysis reveals"
	score, detail := scoreResponse(text, "")
	if detail.ObjectiveTerms < 5 {
		t.Errorf("expected >=5 objective terms, got %d", detail.ObjectiveTerms)
	}
	if detail.ObjectiveScore < 0.5 {
		t.Errorf("expected high objective score, got %f", detail.ObjectiveScore)
	}
	if score < 50 {
		t.Errorf("expected score > 50 with many objective terms, got %f", score)
	}
}

func TestScoreResponseBias(t *testing.T) {
	text := "Obviously clearly undoubtedly the best amazing fantastic brilliant perfect"
	_, detail := scoreResponse(text, "")
	if detail.BiasTerms < 5 {
		t.Errorf("expected >=5 bias terms, got %d", detail.BiasTerms)
	}
	if detail.BiasPenalty < 0.5 {
		t.Errorf("expected high bias penalty, got %f", detail.BiasPenalty)
	}
}

func TestScoreResponseBalance(t *testing.T) {
	text := "On the other hand however conversely alternatively some argue others suggest"
	_, detail := scoreResponse(text, "")
	if detail.BalanceScore < 0.5 {
		t.Errorf("expected high balance score, got %f", detail.BalanceScore)
	}
}

func TestScoreResponseTone(t *testing.T) {
	text := "It appears the evidence suggests it is worth noting one consideration a key factor an important aspect"
	_, detail := scoreResponse(text, "")
	if detail.ToneScore < 0.5 {
		t.Errorf("expected high tone score, got %f", detail.ToneScore)
	}
}

func TestScoreResponseEvidence(t *testing.T) {
	text := "For example such as including demonstrated by as shown by illustrated by cited in"
	_, detail := scoreResponse(text, "")
	if detail.EvidenceScore < 0.5 {
		t.Errorf("expected high evidence score, got %f", detail.EvidenceScore)
	}
}

func TestScoreResponseBounded(t *testing.T) {
	huge := ""
	for i := 0; i < 100; i++ {
		huge += "according to research shows studies indicate for example "
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
		makeFinding("gpt-4", "lab1", "According to research shows the evidence suggests", nil),
		makeFinding("gpt-4", "lab2", "However on the other hand some argue others suggest", nil),
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
		makeFinding("model-a", "lab1", "According to research shows evidence points objectively statistically", nil),
		makeFinding("model-b", "lab1", "Obviously clearly the best amazing terrible stupid", nil),
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
		makeFinding("m1", "lab1", "obviously clearly undoubtedly the best amazing horrible stupid ridiculous", nil),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns for biased response")
	}
}

func TestAnalyzeSorted(t *testing.T) {
	findings := []model.Finding{
		makeFinding("bad", "lab1", "obviously terrible stupid ridiculous", nil),
		makeFinding("good", "lab1", "according to research shows evidence points objectively the evidence suggests", nil),
	}
	r := Analyze(findings)
	if r.Models[0].NeutralityScore < r.Models[1].NeutralityScore {
		t.Error("models should be sorted by score descending")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Report{}
	out := FormatText(r)
	if out == "" {
		t.Error("expected non-empty output")
	}
	if !contains(out, "No neutrality data") {
		t.Error("expected 'No neutrality data' message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		makeFinding("test-model", "lab1", "According to research shows the evidence suggests", nil),
	}
	r := Analyze(findings)
	out := FormatText(r)
	if !contains(out, "Response Neutrality") {
		t.Error("expected header")
	}
	if !contains(out, "test-model") {
		t.Error("expected model name")
	}
	if !contains(out, "objective_score") {
		t.Error("expected detail labels")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		makeFinding("m1", "lab1", "According to research", nil),
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
	count := countIndicators("", objectiveIndicators)
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}
