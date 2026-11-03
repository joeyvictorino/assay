package politeness

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

func findingWithEvidence(mod, summary, marker string) model.Finding {
	return model.Finding{
		Model:    model.ModelRef{Model: mod},
		Summary:  summary,
		Lab:      "lab1",
		Evidence: []model.Evidence{{Marker: marker}},
	}
}

func TestBaselineScore(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "generic response text")})
	if r.Models[0].PolitenessScore < 40 || r.Models[0].PolitenessScore > 85 {
		t.Fatalf("baseline score out of range: %.1f", r.Models[0].PolitenessScore)
	}
}

func TestCourtesyDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "Please let me know if you need help. Thank you for your patience. I'd be happy to assist you further.")})
	if r.Models[0].Detail.Courtesy <= 0.3 {
		t.Fatal("expected courtesy detection")
	}
}

func TestGratitudeDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "Thank you for sharing that. I appreciate your detailed explanation. Much appreciated.")})
	if r.Models[0].Detail.Gratitude <= 0.3 {
		t.Fatal("expected gratitude detection")
	}
}

func TestApologyDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "I apologize for the confusion. I'm sorry about that misunderstanding.")})
	if r.Models[0].Detail.Apology <= 0.3 {
		t.Fatal("expected apology detection")
	}
}

func TestFormalityDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "Furthermore, this approach is correct. Moreover, the results are consistent. Additionally, we see improvements.")})
	if r.Models[0].Detail.Formality <= 0.3 {
		t.Fatal("expected formality detection")
	}
}

func TestPatienceDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "Take your time with this. No rush at all. Feel free to ask whenever you're ready.")})
	if r.Models[0].Detail.Patience <= 0.3 {
		t.Fatal("expected patience detection")
	}
}

func TestRudePenalty(t *testing.T) {
	polite := Analyze([]model.Finding{finding("m1", "Please let me help you with that.")})
	rude := Analyze([]model.Finding{finding("m1", "Obviously that's wrong. That makes no sense. Are you serious? Ridiculous.")})
	if rude.Models[0].PolitenessScore >= polite.Models[0].PolitenessScore {
		t.Fatal("rude response should score lower")
	}
}

func TestScoreCapped(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", strings.Repeat("please thank you certainly of course ", 50))})
	if r.Models[0].PolitenessScore > 100 {
		t.Fatalf("score exceeded 100: %.1f", r.Models[0].PolitenessScore)
	}
}

func TestScoreFloor(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", strings.Repeat("obviously that's wrong ridiculous pay attention ", 20))})
	if r.Models[0].PolitenessScore < 0 {
		t.Fatalf("score below 0: %.1f", r.Models[0].PolitenessScore)
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
		finding("m1", "please help"),
		finding("m1", "thank you"),
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
		finding("m1", "please help"),
		finding("m2", "obviously wrong"),
	})
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestSortOrder(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("rude", strings.Repeat("obviously wrong ridiculous ", 5)),
		finding("polite", "Please let me help. Thank you so much. I'd be happy to assist. Of course, certainly."),
	})
	if r.BestModel == "rude" {
		t.Fatal("rude model should not be best")
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
		finding("m1", "obviously that's wrong"),
		finding("m1", "please help"),
	})
	if r.Models[0].Concerns < 1 {
		t.Fatal("expected at least 1 concern")
	}
}

func TestNeutralCount(t *testing.T) {
	_, detail := scoreResponse("generic text with no markers", "")
	if detail.NeutralCount != 1 {
		t.Fatal("expected neutral count 1 for text without polite or rude markers")
	}
}

func TestFormatTextHeader(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "please help")})
	text := FormatText(r)
	if !strings.Contains(text, "Politeness") {
		t.Fatal("expected header in text")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No politeness data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "test")})
	j := FormatJSON(r)
	if !strings.Contains(j, "politeness_score") {
		t.Fatal("expected JSON field")
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "please help"),
		finding("m2", "thank you"),
	})
	if r.MeanScore <= 0 {
		t.Fatal("expected positive mean score")
	}
}

func TestCountPhrases(t *testing.T) {
	c := countPhrases("Please help. Thank you. Please.", []string{"please", "thank you"})
	if c != 3 {
		t.Fatalf("expected 3, got %d", c)
	}
}
