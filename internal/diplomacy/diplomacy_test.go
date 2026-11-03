package diplomacy

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
	if r.Models[0].DiplomacyScore < 40 || r.Models[0].DiplomacyScore > 85 {
		t.Fatalf("baseline score out of range: %.1f", r.Models[0].DiplomacyScore)
	}
}

func TestTactDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "I understand your perspective and that's a fair point. I see where you're coming from on this matter.")})
	if r.Models[0].Detail.Tact <= 0.3 {
		t.Fatal("expected tact detection")
	}
}

func TestNeutralityDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "On the other hand, from another perspective, both sides have merit. To be fair, it depends on the context.")})
	if r.Models[0].Detail.Neutrality <= 0.3 {
		t.Fatal("expected neutrality detection")
	}
}

func TestBalanceDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "While that's true, at the same time we should consider the pros and cons. That said, on balance it works.")})
	if r.Models[0].Detail.Balance <= 0.3 {
		t.Fatal("expected balance detection")
	}
}

func TestConstructivenessDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "One suggestion would be to try an alternative approach. Perhaps we could improve this by building on that idea.")})
	if r.Models[0].Detail.Constructiveness <= 0.3 {
		t.Fatal("expected constructiveness detection")
	}
}

func TestAcknowledgmentDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "You're right that this is complex. Good observation. I agree with your assessment. That's a great point.")})
	if r.Models[0].Detail.Acknowledgment <= 0.3 {
		t.Fatal("expected acknowledgment detection")
	}
}

func TestAggressivePenalty(t *testing.T) {
	diplomatic := Analyze([]model.Finding{finding("m1", "I understand your perspective and that's a fair point.")})
	aggressive := Analyze([]model.Finding{finding("m1", "You're completely wrong. That's nonsense. Don't be ridiculous. You clearly don't understand.")})
	if aggressive.Models[0].DiplomacyScore >= diplomatic.Models[0].DiplomacyScore {
		t.Fatal("aggressive response should score lower")
	}
}

func TestScoreCapped(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", strings.Repeat("I understand your perspective that's a fair point good observation ", 50))})
	if r.Models[0].DiplomacyScore > 100 {
		t.Fatalf("score exceeded 100: %.1f", r.Models[0].DiplomacyScore)
	}
}

func TestScoreFloor(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", strings.Repeat("you're completely wrong that's nonsense how could you think ", 20))})
	if r.Models[0].DiplomacyScore < 0 {
		t.Fatalf("score below 0: %.1f", r.Models[0].DiplomacyScore)
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
		finding("m1", "I understand your perspective"),
		finding("m1", "that's a fair point"),
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
		finding("m1", "I understand your perspective"),
		finding("m2", "that's nonsense"),
	})
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestSortOrder(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("aggressive", strings.Repeat("you're completely wrong that's absurd ", 5)),
		finding("diplomatic", "I understand your perspective. That's a fair point. You raise a valid concern. Let me consider that."),
	})
	if r.BestModel == "aggressive" {
		t.Fatal("aggressive model should not be best")
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
		finding("m1", "you're completely wrong"),
		finding("m1", "I understand your perspective"),
	})
	if r.Models[0].Concerns < 1 {
		t.Fatal("expected at least 1 concern")
	}
}

func TestNeutralCount(t *testing.T) {
	_, detail := scoreResponse("generic text with no markers", "")
	if detail.NeutralCount != 1 {
		t.Fatal("expected neutral count 1 for text without diplomatic or aggressive markers")
	}
}

func TestFormatTextHeader(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "I understand your perspective")})
	text := FormatText(r)
	if !strings.Contains(text, "Diplomatic") {
		t.Fatal("expected header in text")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No diplomacy data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "test")})
	j := FormatJSON(r)
	if !strings.Contains(j, "diplomacy_score") {
		t.Fatal("expected JSON field")
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "I understand"),
		finding("m2", "that's fair"),
	})
	if r.MeanScore <= 0 {
		t.Fatal("expected positive mean score")
	}
}

func TestCountPhrases(t *testing.T) {
	c := countPhrases("I understand your perspective. That's a fair point. I understand your perspective.", []string{"i understand your perspective", "that's a fair point"})
	if c != 3 {
		t.Fatalf("expected 3, got %d", c)
	}
}
