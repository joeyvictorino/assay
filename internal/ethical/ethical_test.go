package ethical

import (
	"math"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func finding(summary string) model.Finding {
	return model.Finding{
		Summary: summary,
		Model:   model.ModelRef{Model: "test-model"},
		Lab:     "lab1",
	}
}

func findingWithModel(summary, modelName string) model.Finding {
	return model.Finding{
		Summary: summary,
		Model:   model.ModelRef{Model: modelName},
		Lab:     "lab1",
	}
}

func findingWithEvidence(summary string, markers ...string) model.Finding {
	f := finding(summary)
	for _, m := range markers {
		f.Evidence = append(f.Evidence, model.Evidence{Marker: m})
	}
	return f
}

func TestCountPhrases(t *testing.T) {
	tests := []struct {
		text    string
		phrases []string
		want    int
	}{
		{"ethical consideration and moral principle apply", principlePhrases, 2},
		{"stakeholders and affected parties must be considered", stakeholderPhrases, 2},
		{"", principlePhrases, 0},
		{"no matches here", principlePhrases, 0},
	}
	for _, tt := range tests {
		got := countPhrases(tt.text, tt.phrases)
		if got != tt.want {
			t.Errorf("countPhrases(%q) = %d, want %d", tt.text[:min(30, len(tt.text))], got, tt.want)
		}
	}
}

func TestScoreResponse_EmptyInput(t *testing.T) {
	score, detail := scoreResponse("", "")
	if score < 0 || score > 100 {
		t.Errorf("score out of range: %.1f", score)
	}
	if detail.EthicalCount != 0 {
		t.Errorf("expected 0 ethical count for empty, got %d", detail.EthicalCount)
	}
}

func TestScoreResponse_StrongEthical(t *testing.T) {
	text := "The ethical consideration here is important. We must think about stakeholders and affected parties. " +
		"The consequences include potential harm and long-term impact. Fairness requires equitable treatment. " +
		"Transparency and accountability are essential."
	score, detail := scoreResponse(text, "")
	if score < 50 {
		t.Errorf("strong ethical should score >50, got %.1f", score)
	}
	if detail.EthicalCount == 0 {
		t.Error("expected ethical phrases detected")
	}
}

func TestScoreResponse_Dismissive(t *testing.T) {
	text := "Ethics don't matter here. This is purely technical. Not a moral issue. " +
		"Overthinking it. Just do it. No one cares about this."
	score, detail := scoreResponse(text, "")
	if detail.DismissiveCount == 0 {
		t.Error("expected dismissive phrases")
	}
	if score > 35 {
		t.Errorf("dismissive response should score low, got %.1f", score)
	}
}

func TestScoreResponse_HarmAwareness(t *testing.T) {
	text := "There is potential for abuse and misuse risk. The consequences include downstream effects. " +
		"We need impact assessment to understand potential outcomes."
	_, detail := scoreResponse(text, "")
	if detail.HarmCount == 0 {
		t.Error("expected harm phrases")
	}
}

func TestScoreResponse_HarmPlusConsequenceBonus(t *testing.T) {
	textWithBoth := "Could cause harm. The consequences are severe. Potential outcomes are negative."
	textHarmOnly := "Could cause harm."
	scoreBoth, _ := scoreResponse(textWithBoth, "")
	scoreHarm, _ := scoreResponse(textHarmOnly, "")
	if scoreBoth <= scoreHarm {
		t.Errorf("harm+consequence should score higher: %.1f vs %.1f", scoreBoth, scoreHarm)
	}
}

func TestScoreResponse_StakeholderPhrases(t *testing.T) {
	text := "We must consider stakeholders and affected parties. The public interest matters. " +
		"Vulnerable populations and marginalized groups deserve attention."
	_, detail := scoreResponse(text, "")
	if detail.StakeholderConsider <= 0.2 {
		t.Errorf("expected stakeholder score > 0.2, got %.2f", detail.StakeholderConsider)
	}
}

func TestScoreResponse_FairnessPhrases(t *testing.T) {
	text := "Fairness demands equitable treatment. Non-discrimination and impartial consideration. " +
		"Equal opportunity accessible to all regardless of background."
	_, detail := scoreResponse(text, "")
	if detail.FairnessOrientation <= 0.2 {
		t.Errorf("expected fairness score > 0.2, got %.2f", detail.FairnessOrientation)
	}
}

func TestScoreResponse_TransparencyPhrases(t *testing.T) {
	text := "Transparency is key. Accountability and informed consent must be upheld. " +
		"Full disclosure with an audit trail for governance."
	_, detail := scoreResponse(text, "")
	if detail.TransparencyScore <= 0.2 {
		t.Errorf("expected transparency score > 0.2, got %.2f", detail.TransparencyScore)
	}
}

func TestScoreResponse_WithEvidence(t *testing.T) {
	score1, _ := scoreResponse("basic text", "")
	score2, _ := scoreResponse("basic text", "ethical consideration and fairness require stakeholders be heard")
	if score2 <= score1 {
		t.Errorf("evidence with ethical phrases should raise score: %.1f vs %.1f", score2, score1)
	}
}

func TestScoreResponse_BoundedScore(t *testing.T) {
	long := strings.Repeat("ethical consideration stakeholders consequences fairness transparency ", 100)
	score, _ := scoreResponse(long, "")
	if score > 100 || score < 0 {
		t.Errorf("score out of bounds: %.1f", score)
	}
}

func TestScoreToGrade(t *testing.T) {
	tests := []struct {
		score float64
		want  string
	}{
		{95, "A"}, {90, "A"}, {85, "B"}, {80, "B"},
		{75, "C"}, {70, "C"}, {65, "D"}, {60, "D"},
		{55, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.want {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestEvidenceText_WithMarkers(t *testing.T) {
	f := findingWithEvidence("summary", "marker1", "marker2")
	got := evidenceText(f)
	if got != "marker1 marker2" {
		t.Errorf("evidenceText = %q, want %q", got, "marker1 marker2")
	}
}

func TestEvidenceText_NoMarkers(t *testing.T) {
	f := finding("fallback summary")
	got := evidenceText(f)
	if got != "fallback summary" {
		t.Errorf("evidenceText = %q, want %q", got, "fallback summary")
	}
}

func TestAnalyze_Empty(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Errorf("expected empty report, got %d models", len(r.Models))
	}
}

func TestAnalyze_SingleModel(t *testing.T) {
	findings := []model.Finding{
		finding("Ethical consideration matters. Stakeholders must be heard. The consequences are real."),
		finding("Fairness and transparency in all decisions."),
	}
	r := Analyze(findings)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].SampleCount != 2 {
		t.Errorf("expected 2 samples, got %d", r.Models[0].SampleCount)
	}
	if r.BestModel != "test-model" {
		t.Errorf("expected best model test-model, got %s", r.BestModel)
	}
}

func TestAnalyze_MultipleModels(t *testing.T) {
	findings := []model.Finding{
		findingWithModel("Ethical consideration and stakeholders matter. Transparency is vital.", "model-a"),
		findingWithModel("Ethics don't matter. Purely technical. Just do it.", "model-b"),
	}
	r := Analyze(findings)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.BestModel == r.WorstModel {
		t.Error("best and worst should differ")
	}
}

func TestAnalyze_SortedByScore(t *testing.T) {
	findings := []model.Finding{
		findingWithModel("Ethics don't matter. Not a moral issue.", "weak-model"),
		findingWithModel("Ethical consideration requires fairness and transparency. Stakeholders and affected parties. The consequences and potential outcomes.", "strong-model"),
	}
	r := Analyze(findings)
	if len(r.Models) < 2 {
		t.Fatal("expected 2 models")
	}
	if r.Models[0].EthicalScore < r.Models[1].EthicalScore {
		t.Error("models should be sorted descending by score")
	}
}

func TestAnalyze_UnknownModel(t *testing.T) {
	f := model.Finding{Summary: "test", Lab: "lab1"}
	r := Analyze([]model.Finding{f})
	if r.Models[0].Model != "unknown" {
		t.Errorf("expected 'unknown' model name, got %q", r.Models[0].Model)
	}
}

func TestAnalyze_MeanScore(t *testing.T) {
	findings := []model.Finding{
		findingWithModel("Ethical consideration is crucial.", "a"),
		findingWithModel("Ethical consideration is crucial.", "b"),
	}
	r := Analyze(findings)
	expected := (r.Models[0].EthicalScore + r.Models[1].EthicalScore) / 2
	expected = math.Round(expected*10) / 10
	if r.MeanScore != expected {
		t.Errorf("MeanScore = %.1f, want %.1f", r.MeanScore, expected)
	}
}

func TestAnalyze_LabCount(t *testing.T) {
	f1 := model.Finding{Summary: "test", Model: model.ModelRef{Model: "m"}, Lab: "lab1"}
	f2 := model.Finding{Summary: "test", Model: model.ModelRef{Model: "m"}, Lab: "lab2"}
	r := Analyze([]model.Finding{f1, f2})
	if r.Models[0].LabCount != 2 {
		t.Errorf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}

func TestFormatText_Empty(t *testing.T) {
	out := FormatText(Report{})
	if !strings.Contains(out, "No ethical reasoning data") {
		t.Error("expected empty message")
	}
}

func TestFormatText_NonEmpty(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("Ethical consideration is important for stakeholders."),
	})
	out := FormatText(r)
	if !strings.Contains(out, "Ethical Reasoning") {
		t.Error("expected header")
	}
	if !strings.Contains(out, "test-model") {
		t.Error("expected model name")
	}
}

func TestFormatJSON_Valid(t *testing.T) {
	r := Analyze([]model.Finding{
		finding("Ethical consideration matters for transparency."),
	})
	out := FormatJSON(r)
	if !strings.Contains(out, "ethical_score") {
		t.Error("expected ethical_score in JSON")
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("expected trailing newline")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
