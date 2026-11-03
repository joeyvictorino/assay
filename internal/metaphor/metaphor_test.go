package metaphor

import (
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(modelName, summary string, markers ...string) model.Finding {
	var ev []model.Evidence
	for _, m := range markers {
		ev = append(ev, model.Evidence{Marker: m})
	}
	return model.Finding{
		Model:   model.ModelRef{Model: modelName},
		Lab:     "lab1",
		Summary: summary,
		Evidence: ev,
	}
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatal("expected no models")
	}
}

func TestSingleModel(t *testing.T) {
	fs := []model.Finding{mkFinding("gpt4", "apt metaphor used effectively")}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.BestModel != "gpt4" {
		t.Fatalf("expected best=gpt4, got %s", r.BestModel)
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		mkFinding("gpt4", "apt metaphor illuminating parallel creative metaphor original analogy"),
		mkFinding("claude", "basic response"),
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Fatal("best and worst should differ")
	}
}

func TestAptnessScoring(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "apt metaphor fitting analogy well-chosen comparison illuminating parallel")}
	r := Analyze(fs)
	if r.Models[0].Detail.AptnessCount < 3 {
		t.Fatalf("expected aptness count >= 3, got %d", r.Models[0].Detail.AptnessCount)
	}
}

func TestNoveltyScoring(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "creative metaphor original analogy fresh comparison inventive imagery")}
	r := Analyze(fs)
	if r.Models[0].Detail.NoveltyCount < 3 {
		t.Fatalf("expected novelty count >= 3, got %d", r.Models[0].Detail.NoveltyCount)
	}
}

func TestClarityScoring(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "clear analogy understandable metaphor accessible comparison")}
	r := Analyze(fs)
	if r.Models[0].Detail.ClarityScore <= 0.3 {
		t.Fatalf("expected clarity score > 0.3, got %.2f", r.Models[0].Detail.ClarityScore)
	}
}

func TestExtensionScoring(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "extended metaphor sustained analogy developed comparison")}
	r := Analyze(fs)
	if r.Models[0].Detail.ExtensionScore <= 0.2 {
		t.Fatalf("expected extension score > 0.2, got %.2f", r.Models[0].Detail.ExtensionScore)
	}
}

func TestMixedMetaphorsReduce(t *testing.T) {
	clean := []model.Finding{mkFinding("m1", "apt metaphor well-chosen comparison")}
	mixed := []model.Finding{mkFinding("m2", "apt metaphor well-chosen comparison mixed metaphor conflicting imagery jarring comparison clashing metaphors")}
	r1 := Analyze(clean)
	r2 := Analyze(mixed)
	if r2.Models[0].MetaphorScore >= r1.Models[0].MetaphorScore {
		t.Fatal("mixed metaphors should reduce score")
	}
}

func TestMixedCountConcerns(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "mixed metaphor conflicting imagery inconsistent analogy contradictory figures jarring comparison")}
	r := Analyze(fs)
	if r.Models[0].Detail.MixedCount < 4 {
		t.Fatalf("expected mixed count >= 4, got %d", r.Models[0].Detail.MixedCount)
	}
	if r.Models[0].Concerns < 1 {
		t.Fatal("expected concerns for high mixed count")
	}
}

func TestGradeA(t *testing.T) {
	g := scoreToGrade(92)
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

func TestGradeC(t *testing.T) {
	g := scoreToGrade(72)
	if g != "C" {
		t.Fatalf("expected C, got %s", g)
	}
}

func TestGradeD(t *testing.T) {
	g := scoreToGrade(65)
	if g != "D" {
		t.Fatalf("expected D, got %s", g)
	}
}

func TestGradeF(t *testing.T) {
	g := scoreToGrade(45)
	if g != "F" {
		t.Fatalf("expected F, got %s", g)
	}
}

func TestEvidenceText(t *testing.T) {
	f := mkFinding("m1", "summary text", "marker1", "marker2")
	e := evidenceText(f)
	if !strings.Contains(e, "marker1") {
		t.Fatal("expected marker1 in evidence text")
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := mkFinding("m1", "summary text")
	e := evidenceText(f)
	if e != "summary text" {
		t.Fatalf("expected summary fallback, got %s", e)
	}
}

func TestMeanScore(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "apt metaphor creative metaphor"),
		mkFinding("m2", "basic text"),
	}
	r := Analyze(fs)
	if r.MeanScore <= 0 {
		t.Fatal("expected positive mean score")
	}
}

func TestUnknownModelName(t *testing.T) {
	fs := []model.Finding{{Summary: "apt metaphor", Lab: "lab1"}}
	r := Analyze(fs)
	if r.Models[0].Model != "unknown" {
		t.Fatalf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestLabCount(t *testing.T) {
	fs := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab1", Summary: "test"},
		{Model: model.ModelRef{Model: "m1"}, Lab: "lab2", Summary: "test"},
	}
	r := Analyze(fs)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestFormatTextHeader(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "apt metaphor")}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Metaphor Quality") {
		t.Fatal("expected header in text output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No metaphor data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatJSONContainsModels(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "apt metaphor")}
	r := Analyze(fs)
	j := FormatJSON(r)
	if !strings.Contains(j, "models") {
		t.Fatal("expected models in JSON")
	}
}

func TestScoreBounded(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", strings.Repeat("apt metaphor creative metaphor ", 50))}
	r := Analyze(fs)
	if r.Models[0].MetaphorScore > 100 {
		t.Fatal("score should not exceed 100")
	}
}

func TestScoreNonNegative(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", strings.Repeat("mixed metaphor conflicting imagery ", 50))}
	r := Analyze(fs)
	if r.Models[0].MetaphorScore < 0 {
		t.Fatal("score should not be negative")
	}
}

func TestCountPhrases(t *testing.T) {
	n := countPhrases("creative metaphor and another creative metaphor", []string{"creative metaphor"})
	if n != 2 {
		t.Fatalf("expected 2, got %d", n)
	}
}
