package originality

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFinding(mdl, summary string) model.Finding {
	return model.Finding{
		Model:   model.ModelRef{Model: mdl},
		Summary: summary,
		Lab:     "test-lab",
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatal("expected no models")
	}
}

func TestAnalyzeSingleGeneric(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "The answer is 42.")}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].OriginalityScore <= 0 {
		t.Error("expected positive score")
	}
}

func TestOriginalScoresHigher(t *testing.T) {
	generic := []model.Finding{mkFinding("m1", "As we all know, it goes without saying that everyone knows this.")}
	original := []model.Finding{mkFinding("m1", "A novel approach to this problem involves a unique perspective. Imagine a thought experiment where we rethink the underlying pattern. This counterintuitive insight reveals a hidden connection that is often missed.")}
	rg := Analyze(generic)
	ro := Analyze(original)
	if ro.Models[0].OriginalityScore <= rg.Models[0].OriginalityScore {
		t.Errorf("original (%.1f) should score higher than generic (%.1f)", ro.Models[0].OriginalityScore, rg.Models[0].OriginalityScore)
	}
}

func TestNoveltyScore(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "This novel approach uses an innovative and unconventional creative solution. A fresh take with a new angle.")}
	r := Analyze(fs)
	if r.Models[0].Detail.NoveltyScore < 0.5 {
		t.Errorf("expected higher novelty, got %.2f", r.Models[0].Detail.NoveltyScore)
	}
}

func TestCreativeExpression(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Imagine a scenario where we envision the future. What if we could run a thought experiment? Picture this mental model and consider this analogy.")}
	r := Analyze(fs)
	if r.Models[0].Detail.CreativeExpr < 0.5 {
		t.Errorf("expected higher creative expression, got %.2f", r.Models[0].Detail.CreativeExpr)
	}
}

func TestUniqueInsight(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "A key insight reveals the underlying pattern. This counterintuitive observation shows a hidden connection. The nuanced and subtle point is often missed.")}
	r := Analyze(fs)
	if r.Models[0].Detail.UniqueInsight < 0.5 {
		t.Errorf("expected higher unique insight, got %.2f", r.Models[0].Detail.UniqueInsight)
	}
}

func TestClichePenalty(t *testing.T) {
	clean := []model.Finding{mkFinding("m1", "A novel approach reveals counterintuitive insight through creative expression.")}
	cliched := []model.Finding{mkFinding("m1", "At the end of the day, we need to think outside the box and leverage synergy. Let's deep dive and circle back on the low-hanging fruit.")}
	rc := Analyze(clean)
	rcl := Analyze(cliched)
	if rcl.Models[0].Detail.AvoidsCliche >= rc.Models[0].Detail.AvoidsCliche {
		t.Error("cliched should have lower avoids_cliche score")
	}
}

func TestGenericPenalty(t *testing.T) {
	original := []model.Finding{mkFinding("m1", "Novel perspective with unique creative insight.")}
	generic := []model.Finding{mkFinding("m1", "As we all know, it goes without saying. Obviously everyone knows. Needless to say, of course it's common knowledge.")}
	ro := Analyze(original)
	rg := Analyze(generic)
	if rg.Models[0].Detail.AvoidsCliche >= ro.Models[0].Detail.AvoidsCliche {
		t.Error("generic phrasing should lower the avoids_cliche score")
	}
}

func TestPerspectiveScore(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "On the other hand, from another angle the contrasting view shows differently. Reframing through the lens of the flip side reveals an alternative interpretation.")}
	r := Analyze(fs)
	if r.Models[0].Detail.PerspectiveScore < 0.5 {
		t.Errorf("expected higher perspective score, got %.2f", r.Models[0].Detail.PerspectiveScore)
	}
}

func TestVarietyScore(t *testing.T) {
	varied := []model.Finding{mkFinding("m1", "The quick brown fox jumps over lazy dogs while cats sleep peacefully under warm blankets during cold winter nights.")}
	repetitive := []model.Finding{mkFinding("m1", "the the the the the the the the the the the the the the the the")}
	rv := Analyze(varied)
	rr := Analyze(repetitive)
	if rr.Models[0].Detail.VarietyScore >= rv.Models[0].Detail.VarietyScore {
		t.Error("repetitive should have lower variety score")
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "A novel approach with unique perspective and creative insight."),
		mkFinding("m2", "Generic response."),
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" || r.WorstModel == "" {
		t.Error("expected best/worst set")
	}
}

func TestGradeA(t *testing.T) {
	g := scoreToGrade(95)
	if g != "A" {
		t.Errorf("expected A, got %s", g)
	}
}

func TestGradeB(t *testing.T) {
	g := scoreToGrade(85)
	if g != "B" {
		t.Errorf("expected B, got %s", g)
	}
}

func TestGradeC(t *testing.T) {
	g := scoreToGrade(75)
	if g != "C" {
		t.Errorf("expected C, got %s", g)
	}
}

func TestGradeD(t *testing.T) {
	g := scoreToGrade(65)
	if g != "D" {
		t.Errorf("expected D, got %s", g)
	}
}

func TestGradeF(t *testing.T) {
	g := scoreToGrade(45)
	if g != "F" {
		t.Errorf("expected F, got %s", g)
	}
}

func TestScoreBounded(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", strings.Repeat("novel approach innovative unconventional unique perspective insight counterintuitive imagine envision ", 20))}
	r := Analyze(fs)
	if r.Models[0].OriginalityScore < 0 || r.Models[0].OriginalityScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].OriginalityScore)
	}
}

func TestScoreBoundedLow(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", strings.Repeat("at the end of the day synergy leverage low-hanging fruit as we all know ", 20))}
	r := Analyze(fs)
	if r.Models[0].OriginalityScore < 0 || r.Models[0].OriginalityScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].OriginalityScore)
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Novel and creative response.")}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Originality") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "m1") {
		t.Error("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No originality data") {
		t.Error("expected empty message")
	}
}

func TestFormatJSONValid(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Test response.")}
	r := Analyze(fs)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Error("expected models key")
	}
}

func TestFormatJSONRoundtrip(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestMeanScore(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "Novel approach with creative insight."),
		mkFinding("m2", "Another perspective on the topic."),
	}
	r := Analyze(fs)
	if r.MeanScore <= 0 {
		t.Error("expected positive mean score")
	}
}

func TestLabCount(t *testing.T) {
	fs := []model.Finding{
		{Model: model.ModelRef{Model: "m1"}, Summary: "test", Lab: "lab1"},
		{Model: model.ModelRef{Model: "m1"}, Summary: "test2", Lab: "lab2"},
	}
	r := Analyze(fs)
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestUnknownModel(t *testing.T) {
	fs := []model.Finding{{Summary: "test", Lab: "lab1"}}
	r := Analyze(fs)
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected unknown model, got %s", r.Models[0].Model)
	}
}

func TestEvidenceText(t *testing.T) {
	f := model.Finding{
		Summary: "fallback",
		Evidence: []model.Evidence{
			{Marker: "marker1"},
			{Marker: "marker2"},
		},
	}
	et := evidenceText(f)
	if !strings.Contains(et, "marker1") {
		t.Error("expected marker in evidence text")
	}
}

func TestEvidenceFallback(t *testing.T) {
	f := model.Finding{Summary: "my summary"}
	et := evidenceText(f)
	if et != "my summary" {
		t.Errorf("expected summary fallback, got %q", et)
	}
}

func TestCountUniqueWords(t *testing.T) {
	c := countUniqueWords("the cat sat on the mat")
	if c != 5 {
		t.Errorf("expected 5 unique words, got %d", c)
	}
}
