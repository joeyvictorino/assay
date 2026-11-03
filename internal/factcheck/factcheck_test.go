package factcheck

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func makeFinding(modelName, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     "test-lab",
		Summary: summary,
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestWellCitedResponse(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "According to a peer-reviewed study published in a journal, research indicates that the findings show approximately 85 percent of cases measured at this level."),
	}
	r := Analyze(findings)
	if r.Models[0].FactScore < 60 {
		t.Errorf("well-cited response should score >= 60, got %.1f", r.Models[0].FactScore)
	}
	if r.Models[0].Detail.CitationCues == 0 {
		t.Error("expected citation cues detected")
	}
}

func TestUnverifiableResponse(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Everyone knows this is true. It is well known and common knowledge. Many people say and experts agree that sources say this is widely believed."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.UnverifiableCues == 0 {
		t.Error("expected unverifiable cues detected")
	}
}

func TestAssertedResponse(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "This is certainly true. Definitely, absolutely, undoubtedly this is proven. It is obviously an established fact that is always and 100% guaranteed."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.AssertedClaims == 0 {
		t.Error("expected asserted claims detected")
	}
}

func TestHedgedResponse(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "This might be true. It could possibly indicate something. Perhaps it tends to show this pattern, though it is arguably not certain."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.HedgedClaims == 0 {
		t.Error("expected hedged claims detected")
	}
}

func TestQuantityClaims(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Approximately 45 percent of the 2 million cases were measured at about 30 billion units."),
	}
	r := Analyze(findings)
	if r.Models[0].Detail.QuantityClaims == 0 {
		t.Error("expected quantity claims detected")
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		makeFinding("good-model", "According to research published in a peer-reviewed journal, data suggests approximately 90 percent."),
		makeFinding("bad-model", "Everyone knows this. It is well known and common knowledge. Experts agree many people say this is widely believed."),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostReliable == "" {
		t.Error("expected MostReliable set")
	}
	if r.LeastReliable == "" {
		t.Error("expected LeastReliable set")
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		makeFinding("a", "Simple text."),
		makeFinding("b", "Another text."),
	}
	r := Analyze(findings)
	if r.MeanScore == 0 && len(r.Models) > 0 {
		t.Error("expected non-zero mean score")
	}
}

func TestGradeAssignment(t *testing.T) {
	cases := []struct {
		score float64
		want  string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{35, "F"},
	}
	for _, c := range cases {
		got := scoreToGrade(c.score)
		if got != c.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", c.score, got, c.want)
		}
	}
}

func TestReliabilityCounts(t *testing.T) {
	findings := []model.Finding{
		makeFinding("m", "According to peer-reviewed research published in a journal, the meta-analysis found approximately 95 percent based on data from a university study."),
	}
	r := Analyze(findings)
	if r.Models[0].HighReliability == 0 {
		t.Error("expected high reliability count > 0")
	}
}

func TestEmptyTextScore(t *testing.T) {
	score, _ := scoreResponse("")
	if score != 50 {
		t.Errorf("empty text should score 50, got %.1f", score)
	}
}

func TestFormatTextOutput(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "According to studies this is factual."),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Factual Claim Verification") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "gpt-4") {
		t.Error("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No factual data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSONOutput(t *testing.T) {
	findings := []model.Finding{
		makeFinding("gpt-4", "Research shows facts."),
	}
	r := Analyze(findings)
	j := FormatJSON(r)
	if !strings.Contains(j, "fact_score") {
		t.Error("expected fact_score in JSON")
	}
}

func TestLabCounting(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab-a", Summary: "text"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab-b", Summary: "text"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab-a", Summary: "text"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestUnknownModelFallback(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{}, Lab: "lab", Summary: "some text"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown', got %q", r.Models[0].Model)
	}
}

func TestScoreBoundaries(t *testing.T) {
	score, _ := scoreResponse("everyone knows common knowledge many people say experts agree widely believed sources say it has been said people believe generally accepted everyone knows common knowledge everyone knows")
	if score < 0 || score > 100 {
		t.Errorf("score out of bounds: %.1f", score)
	}
}

func TestCountPhrases(t *testing.T) {
	count := countPhrases("according to research and according to studies", citationCues)
	if count < 2 {
		t.Errorf("expected at least 2, got %d", count)
	}
}
