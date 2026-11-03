package fluency

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
	if r.Models[0].FluencyScore < 40 || r.Models[0].FluencyScore > 85 {
		t.Fatalf("baseline score out of range: %.1f", r.Models[0].FluencyScore)
	}
}

func TestNaturalnessDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "For example, consider this approach. In other words, it simplifies the process. Essentially, it works well.")})
	if r.Models[0].Detail.Naturalness <= 0.4 {
		t.Fatal("expected naturalness detection")
	}
}

func TestGrammarDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "Because this matters, therefore we proceed. However, since conditions changed, consequently results differ.")})
	if r.Models[0].Detail.GrammarScore <= 0.4 {
		t.Fatal("expected grammar detection")
	}
}

func TestFlowDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "First, we set up. Next, we configure. Then, we deploy. Finally, we verify.")})
	if r.Models[0].Detail.FlowScore <= 0.3 {
		t.Fatal("expected flow detection")
	}
}

func TestVocabDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "The nuanced and sophisticated framework provides a comprehensive methodology for understanding the fundamental paradigm.")})
	if r.Models[0].Detail.VocabRange <= 0.3 {
		t.Fatal("expected vocab detection")
	}
}

func TestReadabilityDetection(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "Simply put, this is straightforward. In plain terms, it's easy to understand and intuitive.")})
	if r.Models[0].Detail.Readability <= 0.4 {
		t.Fatal("expected readability detection")
	}
}

func TestAwkwardPenalty(t *testing.T) {
	clean := Analyze([]model.Finding{finding("m1", "For example, this approach works well.")})
	awkward := Analyze([]model.Finding{finding("m1", "As an AI, as a language model, I don't have personal feelings. Based on my training data, my knowledge cutoff prevents me.")})
	if awkward.Models[0].FluencyScore >= clean.Models[0].FluencyScore {
		t.Fatal("awkward response should score lower")
	}
}

func TestFillerPenalty(t *testing.T) {
	clean := Analyze([]model.Finding{finding("m1", "For example, this works.")})
	filler := Analyze([]model.Finding{finding("m1", "It is important to note that it should be noted that needless to say as you may know it goes without saying.")})
	if filler.Models[0].FluencyScore >= clean.Models[0].FluencyScore {
		t.Fatal("filler response should score lower")
	}
}

func TestScoreCapped(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", strings.Repeat("for example in other words specifically furthermore first next ", 50))})
	if r.Models[0].FluencyScore > 100 {
		t.Fatalf("score exceeded 100: %.1f", r.Models[0].FluencyScore)
	}
}

func TestScoreFloor(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", strings.Repeat("as an ai as a language model it is important to note that needless to say ", 20))})
	if r.Models[0].FluencyScore < 0 {
		t.Fatalf("score below 0: %.1f", r.Models[0].FluencyScore)
	}
}

func TestGradeA(t *testing.T) {
	if scoreToGrade(92) != "A" {
		t.Fatal("expected A")
	}
}

func TestGradeC(t *testing.T) {
	if scoreToGrade(75) != "C" {
		t.Fatal("expected C")
	}
}

func TestGradeF(t *testing.T) {
	if scoreToGrade(55) != "F" {
		t.Fatal("expected F")
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
		finding("m1", "for example this works"),
		finding("m1", "in other words it simplifies"),
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
		finding("m1", "for example"),
		finding("m2", "as an ai"),
	})
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
}

func TestSortOrder(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("awkward", strings.Repeat("as an ai as a language model ", 5)),
		finding("fluent", "For example, this is straightforward. In other words, the approach is clear and intuitive."),
	})
	if r.BestModel == "awkward" {
		t.Fatal("awkward model should not be best")
	}
}

func TestLabCount(t *testing.T) {
	fs := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab2"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab3"},
	}
	r := Analyze(fs)
	if r.Models[0].LabCount != 3 {
		t.Fatalf("expected 3 labs, got %d", r.Models[0].LabCount)
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
		Evidence: []model.Evidence{{Marker: "a"}, {Marker: "b"}},
	}
	et := evidenceText(f)
	if et != "a b" {
		t.Fatalf("unexpected: %s", et)
	}
}

func TestEvidenceFallback(t *testing.T) {
	f := model.Finding{Summary: "fallback"}
	if evidenceText(f) != "fallback" {
		t.Fatal("expected fallback")
	}
}

func TestIssuesCounting(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "as an ai language model"),
		finding("m1", "for example this works"),
	})
	if r.Models[0].Issues < 1 {
		t.Fatal("expected at least 1 issue")
	}
}

func TestFormatTextHeader(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "test")})
	text := FormatText(r)
	if !strings.Contains(text, "Fluency") {
		t.Fatal("expected header")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No fluency data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	r := Analyze([]model.Finding{finding("m1", "test")})
	j := FormatJSON(r)
	if !strings.Contains(j, "fluency_score") {
		t.Fatal("expected JSON field")
	}
}

func TestMeanScore(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("m1", "for example"),
		finding("m2", "in other words"),
	})
	if r.MeanScore <= 0 {
		t.Fatal("expected positive mean")
	}
}

func TestCountPhrases(t *testing.T) {
	c := countPhrases("For example, for example, in other words.", []string{"for example", "in other words"})
	if c != 3 {
		t.Fatalf("expected 3, got %d", c)
	}
}
