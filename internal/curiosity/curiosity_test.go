package curiosity

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(mod, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mod},
		Summary: summary,
		Lab:     "lab1",
	}
}

func TestBaselineScore(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "generic response text")})
	if r.Models[0].CuriosityScore < 40 || r.Models[0].CuriosityScore > 85 {
		t.Fatalf("baseline score out of range: %.1f", r.Models[0].CuriosityScore)
	}
}

func TestInquiryDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "That's an interesting question. I wonder what would happen if we approached it differently. Have you considered the alternatives?")})
	if r.Models[0].Detail.Inquiry <= 0.3 {
		t.Fatal("expected inquiry detection")
	}
}

func TestExplorationDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "Let's dig deeper into this. Upon closer examination, a deeper look reveals several patterns. Let me look into the details.")})
	if r.Models[0].Detail.Exploration <= 0.3 {
		t.Fatal("expected exploration detection")
	}
}

func TestOpenMindednessDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "There might be other explanations for this. Another possibility is that multiple factors contribute. We shouldn't rule out alternatives.")})
	if r.Models[0].Detail.OpenMindedness <= 0.3 {
		t.Fatal("expected open-mindedness detection")
	}
}

func TestDepthSeekingDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "The underlying cause is more complex. Fundamentally, the mechanism behind this requires breaking this down to first principles.")})
	if r.Models[0].Detail.DepthSeeking <= 0.3 {
		t.Fatal("expected depth-seeking detection")
	}
}

func TestConnectionDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "This relates to our earlier discussion. Similarly, there's a parallel with the other case. This connects to the broader picture.")})
	if r.Models[0].Detail.Connection <= 0.3 {
		t.Fatal("expected connection detection")
	}
}

func TestDismissivePenalty(t *testing.T) {
	curious := Analyze([]model.Finding{finding("m1", "That's an interesting question. I wonder about the implications. Have you considered alternatives?")})
	dismissive := Analyze([]model.Finding{finding("m1", "That's irrelevant. Not worth exploring. Waste of time. It's obvious. Everyone knows the answer. Case closed.")})
	if dismissive.Models[0].CuriosityScore >= curious.Models[0].CuriosityScore {
		t.Fatal("dismissive response should score lower")
	}
}

func TestScoreCapped(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", strings.Repeat("I wonder what if let's dig deeper the underlying cause this relates to ", 50))})
	if r.Models[0].CuriosityScore > 100 {
		t.Fatalf("score exceeded 100: %.1f", r.Models[0].CuriosityScore)
	}
}

func TestScoreFloor(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", strings.Repeat("that's irrelevant waste of time it's obvious case closed ", 20))})
	if r.Models[0].CuriosityScore < 0 {
		t.Fatalf("score below 0: %.1f", r.Models[0].CuriosityScore)
	}
}

func TestGradeA(t *testing.T) {
	g := scoreToGrade(95)
	if g != "A" {
		t.Fatalf("expected A, got %s", g)
	}
}

func TestGradeB(t *testing.T) {
	g := scoreToGrade(85)
	if g != "B" {
		t.Fatalf("expected B, got %s", g)
	}
}

func TestGradeF(t *testing.T) {
	g := scoreToGrade(50)
	if g != "F" {
		t.Fatalf("expected F, got %s", g)
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatal("expected empty models")
	}
}

func TestSingleModel(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "I wonder about this"),
		finding("m1", "let me explore further"),
	})
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].SampleCount != 2 {
		t.Fatalf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
}

func TestMultiModel(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "I wonder about this"),
		finding("m2", "that's irrelevant"),
	})
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestSortOrder(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("dismissive", strings.Repeat("that's irrelevant waste of time case closed ", 5)),
		finding("curious", "That's an interesting question. I wonder about the underlying cause. Let's dig deeper into this."),
	})
	if r.BestModel == "dismissive" {
		t.Fatal("dismissive model should not be best")
	}
}

func TestLabCount(t *testing.T) {
	fs := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab2"},
	}
	r := Analyze(fs)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestUnknownModel(t *testing.T) {
	r := Analyze([]model.Finding{{Summary: "test", Lab: "lab1"}})
	if r.Models[0].Model != "unknown" {
		t.Fatalf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary:  "summary",
		Evidence: []model.Evidence{{Marker: "marker1"}, {Marker: "marker2"}},
	}
	et := evidenceText(f)
	if et != "marker1 marker2" {
		t.Fatalf("unexpected evidence text: %s", et)
	}
}

func TestEvidenceFallback(t *testing.T) {
	f := model.Finding{Summary: "fallback"}
	et := evidenceText(f)
	if et != "fallback" {
		t.Fatalf("expected fallback, got %s", et)
	}
}

func TestConcernsCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "that's irrelevant"),
		finding("m1", "I wonder about this"),
	})
	if r.Models[0].Concerns < 1 {
		t.Fatal("expected at least 1 concern")
	}
}

func TestNeutralCount(t *testing.T) {
	_, detail := scoreResponse("generic text with no markers", "")
	if detail.NeutralCount != 1 {
		t.Fatal("expected neutral count 1 for text without curious or dismissive markers")
	}
}

func TestFormatTextHeader(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "I wonder about this")})
	text := FormatText(r)
	if !strings.Contains(text, "Curiosity") {
		t.Fatal("expected header in text")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No curiosity data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "test")})
	j := FormatJSON(r)
	if !strings.Contains(j, "curiosity_score") {
		t.Fatal("expected JSON field")
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "I wonder"),
		finding("m2", "let me explore"),
	})
	if r.MeanScore <= 0 {
		t.Fatal("expected positive mean score")
	}
}

func TestCountPhrases(t *testing.T) {
	c := countPhrases("I wonder about this. I wonder about that. What if we tried?", []string{"i wonder", "what if"})
	if c != 3 {
		t.Fatalf("expected 3, got %d", c)
	}
}
