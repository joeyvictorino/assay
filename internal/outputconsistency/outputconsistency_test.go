package outputconsistency

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, lab, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     lab,
		Summary: summary,
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
}

func TestSingleSamplePerLab(t *testing.T) {
	findings := []model.Finding{
		mkFinding("m1", "lab-a", "SQL injection in login"),
		mkFinding("m1", "lab-b", "XSS in search bar"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Grade != "N/A" {
		t.Errorf("single sample per lab should be N/A, got %s", r.Models[0].Grade)
	}
}

func TestHighConsistency(t *testing.T) {
	findings := []model.Finding{
		mkFinding("consistent-model", "lab-a", "SQL injection vulnerability found in the login endpoint"),
		mkFinding("consistent-model", "lab-a", "SQL injection vulnerability detected in the login endpoint"),
		mkFinding("consistent-model", "lab-a", "SQL injection vulnerability identified in the login endpoint"),
	}
	r := Analyze(findings)
	if r.Models[0].ConsistencyScore < 60 {
		t.Errorf("consistent responses should score high, got %.1f", r.Models[0].ConsistencyScore)
	}
}

func TestLowConsistency(t *testing.T) {
	findings := []model.Finding{
		mkFinding("drift-model", "lab-a", "SQL injection vulnerability found in the login endpoint"),
		mkFinding("drift-model", "lab-a", "The application uses quantum computing for blockchain transactions in the neural network optimizer"),
		mkFinding("drift-model", "lab-a", "Weather forecast shows rain tomorrow with high humidity levels across the region"),
	}
	r := Analyze(findings)
	if r.Models[0].ConsistencyScore > 60 {
		t.Errorf("drifting responses should score low, got %.1f", r.Models[0].ConsistencyScore)
	}
}

func TestMultipleModels(t *testing.T) {
	findings := []model.Finding{
		mkFinding("model-a", "lab-x", "XSS vulnerability in search bar"),
		mkFinding("model-a", "lab-x", "XSS vulnerability in search input"),
		mkFinding("model-b", "lab-x", "XSS vulnerability in search"),
		mkFinding("model-b", "lab-x", "Blockchain optimization framework detected"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostConsistent == r.LeastConsistent {
		t.Error("most and least consistent should differ")
	}
}

func TestSemanticOverlapFunc(t *testing.T) {
	s1 := []string{"SQL injection in login", "SQL injection in login endpoint"}
	score := semanticOverlap(s1)
	if score < 40 {
		t.Errorf("similar texts should have high overlap, got %.1f", score)
	}
}

func TestSemanticOverlapDifferent(t *testing.T) {
	s1 := []string{"SQL injection in login", "Weather forecast tomorrow"}
	score := semanticOverlap(s1)
	if score > 30 {
		t.Errorf("different texts should have low overlap, got %.1f", score)
	}
}

func TestFormatStabilityConsistent(t *testing.T) {
	texts := []string{"Simple text output.", "Another simple text.", "Yet another text."}
	score := formatStability(texts)
	if score < 90 {
		t.Errorf("same format should score high, got %.1f", score)
	}
}

func TestFormatStabilityMixed(t *testing.T) {
	texts := []string{
		"Simple text.",
		"- Bullet point\n- Another bullet",
		"1. Numbered list\n2. Second item",
	}
	score := formatStability(texts)
	if score > 50 {
		t.Errorf("mixed formats should score low, got %.1f", score)
	}
}

func TestLengthVarianceConsistent(t *testing.T) {
	texts := []string{
		"SQL injection found in login endpoint via user input",
		"SQL injection detected in login page through user data",
		"SQL injection vulnerability in login form from user input",
	}
	score := lengthVarianceScore(texts)
	if score < 70 {
		t.Errorf("similar lengths should score high, got %.1f", score)
	}
}

func TestLengthVarianceWild(t *testing.T) {
	texts := []string{
		"Short.",
		"This is a much much much much much much much much much much much much much much much much much much much much longer response that goes on and on and on about everything under the sun without stopping",
	}
	score := lengthVarianceScore(texts)
	if score > 40 {
		t.Errorf("wildly different lengths should score low, got %.1f", score)
	}
}

func TestKeywordStabilityConsistent(t *testing.T) {
	texts := []string{
		"SQL injection vulnerability endpoint login",
		"SQL injection detected endpoint login",
		"SQL injection found endpoint login",
	}
	score := keywordStability(texts)
	if score < 50 {
		t.Errorf("overlapping keywords should score reasonably, got %.1f", score)
	}
}

func TestHighDriftCount(t *testing.T) {
	findings := []model.Finding{
		mkFinding("drift-model", "lab-a", "SQL injection in endpoint"),
		mkFinding("drift-model", "lab-a", "Quantum neural blockchain optimizer"),
	}
	r := Analyze(findings)
	if r.Models[0].HighDriftCount < 1 {
		t.Errorf("expected high drift count >= 1, got %d", r.Models[0].HighDriftCount)
	}
}

func TestFormatText(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "lab-a", "test summary one"),
		mkFinding("test-model", "lab-a", "test summary two"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !strings.Contains(text, "Consistency") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "test-model") {
		t.Error("expected model name in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	findings := []model.Finding{
		mkFinding("test-model", "lab-a", "test one"),
		mkFinding("test-model", "lab-a", "test two"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	if !strings.Contains(out, `"consistency_score"`) {
		t.Error("expected consistency_score in JSON output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No consistency data") {
		t.Error("expected empty message")
	}
}

func TestEmptyModelName(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{}, Lab: "lab", Summary: "test"},
		{Model: model.ModelRef{}, Lab: "lab", Summary: "test two"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown' for empty model, got %q", r.Models[0].Model)
	}
}

func TestGradeAssignment(t *testing.T) {
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
	for _, tc := range tests {
		got := scoreToGrade(tc.score)
		if got != tc.grade {
			t.Errorf("scoreToGrade(%.0f) = %q, want %q", tc.score, got, tc.grade)
		}
	}
}

func TestJaccardIdentical(t *testing.T) {
	a := map[string]int{"sql": 1, "injection": 1}
	b := map[string]int{"sql": 1, "injection": 1}
	score := jaccardSimilarity(a, b)
	if score != 100 {
		t.Errorf("identical sets should be 100, got %.1f", score)
	}
}

func TestJaccardDisjoint(t *testing.T) {
	a := map[string]int{"sql": 1, "injection": 1}
	b := map[string]int{"weather": 1, "forecast": 1}
	score := jaccardSimilarity(a, b)
	if score != 0 {
		t.Errorf("disjoint sets should be 0, got %.1f", score)
	}
}

func TestDetectFormatProse(t *testing.T) {
	f := detectFormat("This is a simple sentence about security.")
	if f != "prose" {
		t.Errorf("expected 'prose', got %q", f)
	}
}

func TestDetectFormatBullets(t *testing.T) {
	f := detectFormat("- First item\n- Second item")
	if !strings.Contains(f, "bullets") {
		t.Errorf("expected 'bullets' in format, got %q", f)
	}
}
