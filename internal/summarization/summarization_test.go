package summarization

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(modelName, summary string, evidence ...string) model.Finding {
	f := model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     "test-lab",
		Summary: summary,
	}
	for _, e := range evidence {
		f.Evidence = append(f.Evidence, model.Evidence{Marker: e})
	}
	return f
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestSingleModel(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "The system validates input then returns errors.", "The system first checks all input fields for correctness. If any validation fails, the system returns an appropriate error message to the user. The process involves multiple validation steps including type checking and range verification."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", r.Models[0].Model)
	}
}

func TestCompressionRatio(t *testing.T) {
	long := strings.Repeat("word ", 100)
	short := "Short summary here."
	findings := []model.Finding{
		makeFinding("gpt-4", short, long),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.CompressionRatio >= 0.5 {
		t.Errorf("expected low compression ratio, got %.2f", r.Models[0].Detail.CompressionRatio)
	}
}

func TestHighCompressionPenalty(t *testing.T) {
	source := "Simple input text."
	verbose := strings.Repeat("This is a very long and verbose summary that goes on and on. ", 20)
	findings := []model.Finding{
		makeFinding("gpt-4", verbose, source),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.CompressionRatio <= 1.0 {
		t.Errorf("expected high compression ratio for verbose summary, got %.2f", r.Models[0].Detail.CompressionRatio)
	}
}

func TestHallucinationDetection(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "As mentioned earlier, studies have shown that experts agree the data shows clear results. Research indicates the system works. It is well known that obviously this is correct."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.HallucinatedCount == 0 {
		t.Error("expected hallucination phrases detected")
	}
}

func TestCoherenceDetection(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Therefore the system works. Consequently errors are handled. As a result users are satisfied. In summary the approach is valid. Thus we conclude."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.CoherenceScore == 0 {
		t.Error("expected coherence phrases detected")
	}
}

func TestRedundancyDetection(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "As previously mentioned the system works. To reiterate, it handles errors. As noted, the process is complete. Once more, validation succeeds."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.RedundancyCount == 0 {
		t.Error("expected redundancy phrases detected")
	}
}

func TestFlowDetection(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "First check the input. Then validate. Next process the data. Finally return the result. Additionally log the output. Subsequently archive it."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.FlowScore == 0 {
		t.Error("expected flow phrases detected")
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "First summarize. Then conclude. Therefore done.", "Source text with multiple important details."),
		makeFinding("claude", "Brief summary.", "Source text with multiple important details."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" {
		t.Error("expected best model set")
	}
	if r.WorstModel == "" {
		t.Error("expected worst model set")
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{45, "F"},
	}
	for _, tt := range tests {
		g := scoreToGrade(tt.score)
		if g != tt.grade {
			t.Errorf("score %.0f: expected %s, got %s", tt.score, tt.grade, g)
		}
	}
}

func TestScoreBounded(t *testing.T) {
	hallucinated := strings.Repeat("As mentioned earlier studies have shown experts agree research indicates it is well known obviously clearly stated. ", 20)
	findings := []model.Finding{
		makeFinding("gpt-4", hallucinated),
	}
	r := Analyze(findings)
	if r.Models[0].SummarizationScore < 0 || r.Models[0].SummarizationScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].SummarizationScore)
	}
}

func TestKeyPointCoverage(t *testing.T) {
	source := "The authentication system validates credentials using encrypted tokens. Rate limiting prevents abuse."
	summary := "The authentication system validates credentials using encrypted tokens."
	findings := []model.Finding{
		makeFinding("gpt-4", summary, source),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.KeyPointCoverage == 0 {
		t.Error("expected some key point coverage")
	}
}

func TestMissedKeyPoints(t *testing.T) {
	source := "The comprehensive authentication system validates credentials thoroughly using encrypted tokens with strong encryption algorithms and rate limiting prevents abuse through sophisticated throttling mechanisms."
	summary := "Brief note."
	findings := []model.Finding{
		makeFinding("gpt-4", summary, source),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.MissedKeyPoints == 0 {
		t.Error("expected missed key points for brief summary of detailed source")
	}
}

func TestUnknownModelName(t *testing.T) {
	findings := []model.Finding{
		{
			Model:   model.ModelRef{},
			Lab:     "test-lab",
			Summary: "Test summary.",
		},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestLabCounting(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "gpt-4"}, Lab: "lab-1", Summary: "Summary one."},
		{Model: model.ModelRef{Model: "gpt-4"}, Lab: "lab-2", Summary: "Summary two."},
		{Model: model.ModelRef{Model: "gpt-4"}, Lab: "lab-1", Summary: "Summary three."},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
	if r.Models[0].SampleCount != 3 {
		t.Errorf("expected 3 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestFormatTextNoData(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "Summarization Quality") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "No summarization data") {
		t.Error("expected no-data message")
	}
}

func TestFormatTextWithData(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "First the system validates. Then it processes. Therefore it works."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name in text output")
	}
	if !strings.Contains(text, "key_point_coverage") {
		t.Error("expected detail metrics in text output")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Test summary with therefore and consequently."),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Error("expected 'models' key in JSON")
	}
}

func TestFormatJSONRoundtrip(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if data["mean_score"].(float64) != 0 {
		t.Errorf("expected mean_score 0, got %v", data["mean_score"])
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "First check input. Then validate. Therefore process. Finally return results."),
		makeFinding("claude", "Brief."),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Errorf("expected positive mean score, got %.1f", r.MeanScore)
	}
}

func TestPoorSummaryCount(t *testing.T) {
	bad := strings.Repeat("as mentioned earlier studies have shown experts agree ", 10)
	findings := []model.Finding{
		makeFinding("gpt-4", bad),
	}
	r := Analyze(findings)
	if r.Models[0].PoorSummaries != 1 {
		t.Errorf("expected 1 poor summary, got %d", r.Models[0].PoorSummaries)
	}
}

func TestSortedByScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("bad-model", "x"),
		makeFinding("good-model", "First the system validates input correctly. Then it processes the data efficiently. Therefore the output is reliable. Finally the results are returned. Additionally errors are logged."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.Models[0].SummarizationScore < r.Models[1].SummarizationScore {
		t.Error("expected models sorted by descending score")
	}
}
