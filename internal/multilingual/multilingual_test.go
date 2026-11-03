package multilingual

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(mod, summary string, markers ...string) model.Finding {
	var ev []model.Evidence
	for _, m := range markers {
		ev = append(ev, model.Evidence{Marker: m})
	}
	return model.Finding{
		ID:        "f1",
		Lab:       "lab1",
		Model:     model.ModelRef{Model: mod},
		Summary:   summary,
		Evidence:  ev,
		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}
}

func TestAnalyzeEmpty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatal("expected empty models")
	}
}

func TestAnalyzeSingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "The translation is accurate"),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Fatalf("expected gpt-4, got %s", r.Models[0].Model)
	}
	if r.Models[0].SampleCount != 1 {
		t.Fatalf("expected 1 sample, got %d", r.Models[0].SampleCount)
	}
}

func TestAnalyzeMultipleModels(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "La traducción es correcta y culturalmente apropiada"),
		finding("claude", "The response handles cultural nuance well"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == "" {
		t.Fatal("expected best model to be set")
	}
}

func TestScoreBounded(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "test response with translation quality culturally appropriate tradition custom"),
	}
	r := Analyze(findings)
	if r.Models[0].MultilingualScore < 0 || r.Models[0].MultilingualScore > 100 {
		t.Fatalf("score out of bounds: %f", r.Models[0].MultilingualScore)
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

func TestGradeC(t *testing.T) {
	g := scoreToGrade(75)
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

func TestDetectLanguagesSpanish(t *testing.T) {
	langs := detectLanguages("el gato es grande por la casa")
	found := false
	for _, l := range langs {
		if l == "spanish" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected spanish detection, got %v", langs)
	}
}

func TestDetectLanguagesFrench(t *testing.T) {
	langs := detectLanguages("le chat est dans la maison")
	found := false
	for _, l := range langs {
		if l == "french" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected french detection, got %v", langs)
	}
}

func TestDetectLanguagesEmpty(t *testing.T) {
	langs := detectLanguages("")
	if len(langs) != 0 {
		t.Fatalf("expected no languages, got %v", langs)
	}
}

func TestCountScriptsLatin(t *testing.T) {
	count := countScripts("Hello world")
	if count != 1 {
		t.Fatalf("expected 1 script, got %d", count)
	}
}

func TestCountScriptsMultiple(t *testing.T) {
	count := countScripts("Hello 你好 こんにちは")
	if count < 2 {
		t.Fatalf("expected >=2 scripts, got %d", count)
	}
}

func TestNonASCIIRatioPlainEnglish(t *testing.T) {
	ratio := nonASCIIRatio("Hello world")
	if ratio != 0 {
		t.Fatalf("expected 0 ratio, got %f", ratio)
	}
}

func TestNonASCIIRatioWithUnicode(t *testing.T) {
	ratio := nonASCIIRatio("Hello 世界")
	if ratio <= 0 {
		t.Fatalf("expected positive ratio, got %f", ratio)
	}
}

func TestNonASCIIRatioEmpty(t *testing.T) {
	ratio := nonASCIIRatio("")
	if ratio != 0 {
		t.Fatalf("expected 0, got %f", ratio)
	}
}

func TestCountEncodingIssues(t *testing.T) {
	count := countEncodingIssues("This has â€ encoding problems Ã©")
	if count < 2 {
		t.Fatalf("expected >=2 issues, got %d", count)
	}
}

func TestCountEncodingIssuesClean(t *testing.T) {
	count := countEncodingIssues("This is clean text")
	if count != 0 {
		t.Fatalf("expected 0, got %d", count)
	}
}

func TestCountPhrases(t *testing.T) {
	c := countPhrases("The translation is good, which means it translates well", translationQualityPhrases)
	if c < 2 {
		t.Fatalf("expected >=2, got %d", c)
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if text == "" {
		t.Fatal("expected non-empty text")
	}
	if !containsStr(text, "Multilingual") {
		t.Fatal("expected header")
	}
	if !containsStr(text, "No multilingual data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatTextNonEmpty(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "La traducción es muy buena"),
	}
	r := Analyze(findings)
	text := FormatText(r)
	if !containsStr(text, "gpt-4") {
		t.Fatal("expected model name in output")
	}
	if !containsStr(text, "language_detection") {
		t.Fatal("expected language_detection field")
	}
}

func TestFormatJSONValid(t *testing.T) {
	findings := []model.Finding{
		finding("gpt-4", "Test multilingual response"),
	}
	r := Analyze(findings)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Fatal("expected models key")
	}
}

func TestFormatJSONRoundtrip(t *testing.T) {
	r := Analyze(nil)
	out := FormatJSON(r)
	var data Report
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("roundtrip failed: %v", err)
	}
}

func TestEvidenceTextFromMarkers(t *testing.T) {
	f := model.Finding{
		Summary: "summary",
		Evidence: []model.Evidence{
			{Marker: "marker1"},
			{Marker: "marker2"},
		},
	}
	text := evidenceText(f)
	if text != "marker1 marker2" {
		t.Fatalf("expected 'marker1 marker2', got %q", text)
	}
}

func TestEvidenceTextFallback(t *testing.T) {
	f := model.Finding{Summary: "fallback summary"}
	text := evidenceText(f)
	if text != "fallback summary" {
		t.Fatalf("expected fallback, got %q", text)
	}
}

func TestMeanScore(t *testing.T) {
	findings := []model.Finding{
		finding("m1", "test response one"),
		finding("m2", "test response two"),
	}
	r := Analyze(findings)
	if r.MeanScore <= 0 {
		t.Fatalf("expected positive mean score, got %f", r.MeanScore)
	}
}

func TestLabCounting(t *testing.T) {
	f1 := finding("m1", "test one")
	f1.Lab = "lab-a"
	f2 := finding("m1", "test two")
	f2.Lab = "lab-b"
	r := Analyze([]model.Finding{f1, f2})
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestUnknownModelName(t *testing.T) {
	f := finding("", "test with no model")
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Fatalf("expected 'unknown', got %s", r.Models[0].Model)
	}
}

func TestScoreResponseBasic(t *testing.T) {
	score, detail := scoreResponse("Hello world", "test evidence")
	if score < 0 || score > 100 {
		t.Fatalf("score out of range: %f", score)
	}
	if detail.ScriptCount < 1 {
		t.Fatalf("expected at least 1 script")
	}
}

func containsStr(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && contains(s, sub)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchStr(s, sub)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
