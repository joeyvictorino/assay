package spatial

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(m, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: m},
		Summary: summary,
		Lab:     "test-lab",
	}
}

func TestCountPhrases(t *testing.T) {
	text := "to the left of the object and above the surface adjacent to the wall"
	got := countPhrases(text, positionPhrases)
	if got < 2 {
		t.Errorf("expected >=2 position phrases, got %d", got)
	}
}

func TestCountPhrasesEmpty(t *testing.T) {
	got := countPhrases("", positionPhrases)
	if got != 0 {
		t.Errorf("expected 0 phrases for empty text, got %d", got)
	}
}

func TestCountPhrasesCaseInsensitive(t *testing.T) {
	text := "TO THE LEFT OF and ABOVE THE structure ADJACENT TO"
	got := countPhrases(text, positionPhrases)
	if got < 2 {
		t.Errorf("expected >=2 position phrases (case-insensitive), got %d", got)
	}
}

func TestScoreResponseHighSpatial(t *testing.T) {
	text := "facing north oriented toward to the left of above the the distance between perpendicular to the angle between separated by"
	score, detail := scoreResponse(text, "")
	if score < 50 {
		t.Errorf("expected high-spatial score >=50, got %.1f", score)
	}
	if detail.SpatialCount < 5 {
		t.Errorf("expected spatial count >=5, got %d", detail.SpatialCount)
	}
}

func TestScoreResponseLowSpatial(t *testing.T) {
	text := "the cat sat on the mat"
	score, detail := scoreResponse(text, "")
	if detail.SpatialCount != 0 {
		t.Errorf("expected 0 spatial count, got %d", detail.SpatialCount)
	}
	if score > 70 {
		t.Errorf("expected moderate score for neutral text, got %.1f", score)
	}
}

func TestScoreResponseDisoriented(t *testing.T) {
	text := "contradictory positions spatial inconsistency impossible placement conflicting locations disoriented location paradox"
	score, detail := scoreResponse(text, "")
	if detail.DisorientedCount < 4 {
		t.Errorf("expected disoriented count >=4, got %d", detail.DisorientedCount)
	}
	if score > 50 {
		t.Errorf("expected low score for disoriented text, got %.1f", score)
	}
}

func TestScoreResponseAmbiguous(t *testing.T) {
	text := "somewhere around there in that general area roughly positioned vaguely located not precisely placed approximately there"
	_, detail := scoreResponse(text, "")
	if detail.AmbiguousCount < 4 {
		t.Errorf("expected ambiguous count >=4, got %d", detail.AmbiguousCount)
	}
}

func TestScoreResponseBounds(t *testing.T) {
	score1, _ := scoreResponse("", "")
	if score1 < 0 || score1 > 100 {
		t.Errorf("score out of bounds: %.1f", score1)
	}

	longText := strings.Repeat("facing north to the left of above the perpendicular to ", 100)
	score2, _ := scoreResponse(longText, "")
	if score2 > 100 {
		t.Errorf("score exceeds 100: %.1f", score2)
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
	for _, tc := range tests {
		got := scoreToGrade(tc.score)
		if got != tc.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tc.score, got, tc.grade)
		}
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback summary",
		Evidence: []model.Evidence{
			{Marker: "marker one"},
			{Marker: "marker two"},
		},
	}
	got := evidenceText(f)
	if got != "marker one marker two" {
		t.Errorf("unexpected evidence text: %s", got)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "my summary"}
	got := evidenceText(f)
	if got != "my summary" {
		t.Errorf("expected fallback to summary, got: %s", got)
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected no models for nil input")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "to the left of the object and above the surface facing north"),
		finding("gpt-4", "perpendicular to the wall the angle between the two edges is acute"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.BestModel != "gpt-4" {
		t.Errorf("expected best model gpt-4, got %s", r.BestModel)
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("model-a", "facing north to the left of above the adjacent to perpendicular to the angle between"),
		finding("model-b", "the answer is 42"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Errorf("best and worst should differ with different quality responses")
	}
	if r.Models[0].SpatialScore < r.Models[1].SpatialScore {
		t.Errorf("models should be sorted descending by score")
	}
}

func TestAnalyzeUnknownModel(t *testing.T) {
	findings := []model.Finding{
		{Summary: "to the left of above the adjacent to", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.BestModel != "unknown" {
		t.Errorf("expected unknown model, got %s", r.BestModel)
	}
}

func TestAnalyzeLabCount(t *testing.T) {
	findings := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab2"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
	}
	r := Analyze(findings)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestAnalyzeConcerns(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "contradictory positions spatial inconsistency impossible placement conflicting locations disoriented location paradox conflicting spatial claims spatially impossible"),
	}
	r := Analyze(findings)
	if r.Models[0].Concerns < 1 {
		t.Errorf("expected concerns for disoriented response")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Report{}
	text := FormatText(r)
	if !strings.Contains(text, "No spatial reasoning data") {
		t.Errorf("expected empty message in text output")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("test-model", "to the left of above the facing north perpendicular to"),
	})
	text := FormatText(r)
	if !strings.Contains(text, "test-model") {
		t.Errorf("expected model name in text output")
	}
	if !strings.Contains(text, "spatial_orientation") {
		t.Errorf("expected detail fields in text output")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "to the left of and above the facing north"),
	})
	j := FormatJSON(r)
	if !strings.Contains(j, "spatial_score") {
		t.Errorf("expected spatial_score in JSON output")
	}
	if !strings.Contains(j, "spatial_orientation") {
		t.Errorf("expected spatial_orientation in JSON output")
	}
}

func TestMeanScoreCalculation(t *testing.T) {
	findings := []model.Finding{
		finding("a", "facing north to the left of above the perpendicular to"),
		finding("b", "the cat sat on the mat"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Errorf("expected positive mean score, got %.1f", r.MeanScore)
	}
}

func TestGeometricDetection(t *testing.T) {
	text := "the angle between perpendicular to parallel to the triangle formed circular path symmetrical about"
	_, detail := scoreResponse(text, "")
	if detail.GeometricReasoning < 0.3 {
		t.Errorf("expected higher geometric reasoning score, got %.2f", detail.GeometricReasoning)
	}
}
