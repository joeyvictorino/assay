package numeracy

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(modelName, summary string, markers ...string) model.Finding {
	f := model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Summary: summary,
		Lab:     "test-lab",
	}
	for _, m := range markers {
		f.Evidence = append(f.Evidence, model.Evidence{Marker: m})
	}
	return f
}

func TestCountPhrases(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		phrases []string
		want    int
	}{
		{"empty text", "", quantitativePhrases, 0},
		{"no match", "hello world", quantitativePhrases, 0},
		{"single match", "the ratio is 3:1", quantitativePhrases, 1},
		{"multiple matches", "the ratio averaged at a total of 50 percent", quantitativePhrases, 4},
		{"case insensitive", "PERCENT RATIO SCORED", quantitativePhrases, 3},
		{"repeated phrase", "percent percent percent", quantitativePhrases, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countPhrases(tt.text, tt.phrases)
			if got != tt.want {
				t.Errorf("countPhrases() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestScoreResponseStrongNumeric(t *testing.T) {
	text := "The response time averaged 250 milliseconds with a standard deviation " +
		"of 15 milliseconds. Exactly 95 percent of requests completed within 300 milliseconds. " +
		"Compared to the baseline of 400 milliseconds, this represents a 37.5 percent improvement. " +
		"The confidence interval is narrow with a p-value below 0.01."
	score, detail := scoreResponse(text, "")
	if score < 60 {
		t.Errorf("strong numeric text scored only %.1f, want >= 60", score)
	}
	if detail.NumericCount == 0 {
		t.Error("expected non-zero numeric count")
	}
	if detail.VagueCount != 0 {
		t.Errorf("expected zero vague count, got %d", detail.VagueCount)
	}
}

func TestScoreResponseVague(t *testing.T) {
	text := "There are a lot of issues. Many problems were found. " +
		"Some endpoints have a number of failures. " +
		"Several services show various errors. " +
		"Roughly around a substantial amount of traffic."
	score, detail := scoreResponse(text, "")
	if detail.VagueCount == 0 {
		t.Error("expected vague phrases to be detected")
	}
	if score > 60 {
		t.Errorf("vague text scored %.1f, want <= 60", score)
	}
}

func TestScoreResponseInconsistent(t *testing.T) {
	text := "Actually the number is different. I need to correct my calculation was wrong. " +
		"Let me recalculate because it does not add up. The previous figure was off by a factor."
	score, detail := scoreResponse(text, "")
	if detail.InconsistentCount == 0 {
		t.Error("expected inconsistent phrases to be detected")
	}
	if score > 50 {
		t.Errorf("inconsistent text scored %.1f, want <= 50", score)
	}
}

func TestScoreResponseEmpty(t *testing.T) {
	score, detail := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Errorf("empty response score %.1f out of range", score)
	}
	if detail.NumericCount != 0 {
		t.Errorf("expected zero numeric count for empty, got %d", detail.NumericCount)
	}
}

func TestScoreResponseWithEvidence(t *testing.T) {
	response := "The issue exists"
	evidence := "measured at 50 percent with a ratio of 3:1 in milliseconds compared to baseline"
	score, _ := scoreResponse(response, evidence)
	scoreNoEvidence, _ := scoreResponse(response, "")
	if score <= scoreNoEvidence {
		t.Errorf("evidence should improve score: with=%.1f, without=%.1f", score, scoreNoEvidence)
	}
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
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestScoreClampedTo100(t *testing.T) {
	text := strings.Repeat("percent ratio measured at exactly within the range milliseconds compared to confidence level ", 50)
	score, _ := scoreResponse(text, "")
	if score > 100 {
		t.Errorf("score %.1f exceeds 100", score)
	}
}

func TestScoreClampedTo0(t *testing.T) {
	text := strings.Repeat("a lot many few some a number of several various quite a bit substantial amount roughly around about in the ballpark give or take actually the number is i need to correct my calculation was wrong let me recalculate does not add up ", 10)
	score, _ := scoreResponse(text, "")
	if score < 0 {
		t.Errorf("score %.1f below 0", score)
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{Summary: "test summary"}
	if evidenceText(f) != "test summary" {
		t.Error("expected summary as fallback")
	}
	f.Evidence = []model.Evidence{{Marker: "marker1"}, {Marker: "marker2"}}
	got := evidenceText(f)
	if got != "marker1 marker2" {
		t.Errorf("evidenceText() = %q, want %q", got, "marker1 marker2")
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Error("expected empty report for nil findings")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "The ratio averaged 50 percent with exactly 100 requests per second"),
		finding("gpt-4", "Measured at 200 milliseconds compared to the baseline of 300 milliseconds"),
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
	if r.BestModel != "gpt-4" || r.WorstModel != "gpt-4" {
		t.Error("best and worst should both be gpt-4")
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("model-a", "Exactly 95 percent of requests averaged 250 milliseconds with confidence interval"),
		finding("model-b", "A lot of issues. Many problems. Some failures roughly around there."),
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
		{Summary: "the ratio is 50 percent", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown model, got %s", r.Models[0].Model)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "percent ratio"),
		{Model: model.ModelRef{Model: "m1"}, Summary: "measured at", Lab: "lab2"},
	}
	findings[0].Lab = "lab1"
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeSortedByScore(t *testing.T) {
	findings := []model.Finding{
		finding("bad-model", "a lot of many few some various roughly around"),
		finding("good-model", "exactly 95 percent averaged 250 milliseconds compared to baseline confidence interval"),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].NumeracyScore < r.Models[1].NumeracyScore {
		t.Error("models should be sorted descending by score")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	s := FormatText(Report{})
	if !strings.Contains(s, "No numeracy data") {
		t.Error("expected no-data message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "the ratio is 50 percent measured at 100 milliseconds"),
	})
	s := FormatText(r)
	if !strings.Contains(s, "Numerical Accuracy") {
		t.Error("expected header")
	}
	if !strings.Contains(s, "test-model") {
		t.Error("expected model name")
	}
	if !strings.Contains(s, "quantitative_use") {
		t.Error("expected detail field")
	}
}

func TestFormatJSONValid(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "50 percent ratio"),
	})
	s := FormatJSON(r)
	var parsed Report
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(parsed.Models) != 1 {
		t.Errorf("expected 1 model in JSON, got %d", len(parsed.Models))
	}
}

func TestFormatJSONEmpty(t *testing.T) {
	s := FormatJSON(Report{})
	var parsed Report
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "the ratio averaged 50 percent"),
		finding("m2", "measured at 100 milliseconds"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestConcernsCount(t *testing.T) {
	text := "a lot of many issues. Few problems found. Some endpoints have " +
		"a number of errors. Several services show various failures. " +
		"Roughly around quite a bit of issues. Substantial amount of traffic."
	findings := []model.Finding{finding("m1", text)}
	r := Analyze(findings)
	if r.Models[0].Concerns == 0 {
		t.Error("expected concerns for highly vague text")
	}
}
