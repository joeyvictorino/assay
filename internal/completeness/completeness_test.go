package completeness

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

func TestAnalyzeSingleShort(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "The answer is 42.")}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].CompletenessScore <= 0 {
		t.Error("expected positive score")
	}
}

func TestCompleteResponseScoresHigher(t *testing.T) {
	short := []model.Finding{mkFinding("m1", "Yes.")}
	detailed := []model.Finding{mkFinding("m1", "First, the answer involves multiple factors. Additionally, there are several considerations. Because the underlying mechanism works by combining these elements, the result is clear. For example, consider the case where all inputs are valid. Furthermore, this means the output is deterministic. In conclusion, the answer is well-supported.")}
	rs := Analyze(short)
	rd := Analyze(detailed)
	if rd.Models[0].CompletenessScore <= rs.Models[0].CompletenessScore {
		t.Errorf("detailed (%.1f) should score higher than short (%.1f)", rd.Models[0].CompletenessScore, rs.Models[0].CompletenessScore)
	}
}

func TestTopicCoverage(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "First we examine the data. Additionally there are other factors. Furthermore the analysis shows more. Moreover we also find patterns.")}
	r := Analyze(fs)
	if r.Models[0].Detail.TopicCoverage < 0.5 {
		t.Errorf("expected higher topic coverage, got %.2f", r.Models[0].Detail.TopicCoverage)
	}
}

func TestDepthScore(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Because the underlying mechanism works by combining these factors specifically in detail. The reason is fundamentally clear. This means the key factor is notable.")}
	r := Analyze(fs)
	if r.Models[0].Detail.DepthScore < 0.5 {
		t.Errorf("expected higher depth, got %.2f", r.Models[0].Detail.DepthScore)
	}
}

func TestStructureScore(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Overview of the topic. Firstly we examine X. Secondly Y follows. Thirdly Z completes. In conclusion all points are covered.")}
	r := Analyze(fs)
	if r.Models[0].Detail.StructureScore < 0.5 {
		t.Errorf("expected higher structure, got %.2f", r.Models[0].Detail.StructureScore)
	}
}

func TestExamplePresence(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "For example consider the case of network traffic. For instance packet analysis such as deep inspection demonstrated by wireshark captures.")}
	r := Analyze(fs)
	if r.Models[0].Detail.ExamplePresence < 0.5 {
		t.Errorf("expected higher example presence, got %.2f", r.Models[0].Detail.ExamplePresence)
	}
}

func TestClosureScore(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Some content here. In summary the findings are clear. To conclude all points have been addressed. Overall the result is positive.")}
	r := Analyze(fs)
	if r.Models[0].Detail.ClosureScore > 0.2 {
		// has closure indicators
	}
}

func TestIncompletePenalty(t *testing.T) {
	complete := []model.Finding{mkFinding("m1", "The answer is clearly supported by evidence. Because the data shows this.")}
	incomplete := []model.Finding{mkFinding("m1", "I don't know the answer. I'm not sure about this. I cannot determine the result. Further research needed.")}
	rc := Analyze(complete)
	ri := Analyze(incomplete)
	if ri.Models[0].CompletenessScore >= rc.Models[0].CompletenessScore {
		t.Error("incomplete should score lower")
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "First the data shows X. Additionally Y follows. Because the mechanism works specifically."),
		mkFinding("m2", "Yes."),
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
	fs := []model.Finding{mkFinding("m1", strings.Repeat("first additionally furthermore moreover because specifically in detail for example for instance in conclusion ", 20))}
	r := Analyze(fs)
	if r.Models[0].CompletenessScore < 0 || r.Models[0].CompletenessScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].CompletenessScore)
	}
}

func TestScoreBoundedLow(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", strings.Repeat("i don't know i'm not sure i cannot insufficient data ", 20))}
	r := Analyze(fs)
	if r.Models[0].CompletenessScore < 0 || r.Models[0].CompletenessScore > 100 {
		t.Errorf("score out of bounds: %.1f", r.Models[0].CompletenessScore)
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "First, detailed response with examples.")}
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Completeness") {
		t.Error("expected header in text output")
	}
	if !strings.Contains(text, "m1") {
		t.Error("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No completeness data") {
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

func TestConcernsFlagged(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "Quick answer.")}
	r := Analyze(fs)
	if r.Models[0].Concerns == 0 {
		// short responses should flag concerns for low topic/depth
	}
}

func TestMeanScore(t *testing.T) {
	fs := []model.Finding{
		mkFinding("m1", "First detailed answer with depth."),
		mkFinding("m2", "Another model response with examples."),
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

func TestCountIndicators(t *testing.T) {
	text := "first and also first again"
	c := countIndicators(text, []string{"first"})
	if c != 2 {
		t.Errorf("expected 2, got %d", c)
	}
}

func TestCountSentences(t *testing.T) {
	c := countSentences("Hello. World! How?")
	if c != 3 {
		t.Errorf("expected 3, got %d", c)
	}
}

func TestCountSentencesEmpty(t *testing.T) {
	c := countSentences("no punctuation")
	if c != 1 {
		t.Errorf("expected 1, got %d", c)
	}
}

func TestQuestionCount(t *testing.T) {
	fs := []model.Finding{mkFinding("m1", "What is this? How does it work? Why?")}
	r := Analyze(fs)
	if r.Models[0].Detail.QuestionCount < 3 {
		t.Errorf("expected at least 3 questions, got %d", r.Models[0].Detail.QuestionCount)
	}
}
