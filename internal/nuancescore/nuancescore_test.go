package nuancescore

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func mkFindings(summaries ...string) []model.Finding {
	var fs []model.Finding
	for i, s := range summaries {
		fs = append(fs, model.Finding{
			ID:      s,
			Lab:     "lab-1",
			Summary: s,
			Model:   model.ModelRef{Model: "test-model"},
			Location: model.Location{
				PathTemplate: func() string {
					if i%2 == 0 {
						return "Is X always better than Y?"
					}
					return "What are the tradeoffs?"
				}(),
			},
		})
	}
	return fs
}

func TestEmptyFindings(t *testing.T) {
	r := Analyze(nil)
	if len(r.Models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(r.Models))
	}
	if r.MeanScore != 0 {
		t.Fatalf("expected 0 mean, got %.1f", r.MeanScore)
	}
}

func TestSingleFinding(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1", Summary: "X is better than Y",
			Model: model.ModelRef{Model: "gpt-4"},
		},
	}
	r := Analyze(fs)
	if len(r.Models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(r.Models))
	}
	if r.Models[0].Model != "gpt-4" {
		t.Fatalf("expected gpt-4, got %s", r.Models[0].Model)
	}
}

func TestHighNuanceResponse(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary: "However, this is debated. Some argue one thing, while others believe differently. It depends on the context. There are multiple viewpoints and the evidence is mixed. Perhaps in some cases X is better, although not always.",
			Model:   model.ModelRef{Model: "claude"},
		},
	}
	r := Analyze(fs)
	if r.Models[0].NuanceScore < 60 {
		t.Fatalf("expected high nuance score, got %.1f", r.Models[0].NuanceScore)
	}
}

func TestLowNuanceResponse(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary: "X is always better. This is certainly true without exception. It is absolutely guaranteed and obviously correct. There is no debate.",
			Model:   model.ModelRef{Model: "gpt-3"},
		},
	}
	r := Analyze(fs)
	if r.Models[0].NuanceScore >= 50 {
		t.Fatalf("expected low nuance score, got %.1f", r.Models[0].NuanceScore)
	}
}

func TestCaveatDetection(t *testing.T) {
	_, detail := scoreResponse("However, this is true. Although there are exceptions, it's worth noting the caveats.")
	if detail.CaveatCount < 3 {
		t.Fatalf("expected at least 3 caveats, got %d", detail.CaveatCount)
	}
}

func TestHedgingDetection(t *testing.T) {
	_, detail := scoreResponse("It might work. It could possibly help. This probably tends to be true generally.")
	if detail.HedgingPhrases < 3 {
		t.Fatalf("expected at least 3 hedging phrases, got %d", detail.HedgingPhrases)
	}
}

func TestAbsoluteClaimDetection(t *testing.T) {
	_, detail := scoreResponse("This is always true. It will never fail. Certainly this is guaranteed without exception.")
	if detail.AbsoluteClaims < 3 {
		t.Fatalf("expected at least 3 absolute claims, got %d", detail.AbsoluteClaims)
	}
}

func TestPerspectiveDetection(t *testing.T) {
	_, detail := scoreResponse("Some argue X is correct. Others believe Y is better. There is ongoing debate with different schools of thought and proponents on each side.")
	if detail.PerspectiveCount < 3 {
		t.Fatalf("expected at least 3 perspectives, got %d", detail.PerspectiveCount)
	}
}

func TestFalseDichotomyDetection(t *testing.T) {
	_, detail := scoreResponse("Either you choose A or B. It's either one or the other. There is no middle ground.")
	if detail.FalseDichotomies < 2 {
		t.Fatalf("expected at least 2 false dichotomies, got %d", detail.FalseDichotomies)
	}
}

func TestUncertaintyDetection(t *testing.T) {
	_, detail := scoreResponse("This is uncertain and debated. The evidence is inconclusive with mixed evidence from ongoing research.")
	if detail.UncertaintyAcknowledged < 3 {
		t.Fatalf("expected at least 3 uncertainty phrases, got %d", detail.UncertaintyAcknowledged)
	}
}

func TestMultipleModels(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary: "However, this depends on context. Some argue one way, others believe differently.",
			Model:   model.ModelRef{Model: "claude"},
		},
		{
			ID: "f2", Lab: "lab-1",
			Summary: "This is always correct without exception.",
			Model:   model.ModelRef{Model: "gpt-4"},
		},
	}
	r := Analyze(fs)
	if len(r.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(r.Models))
	}
	if r.MostNuanced == r.LeastNuanced {
		t.Fatal("most and least nuanced should differ")
	}
}

func TestGrading(t *testing.T) {
	tests := []struct {
		score float64
		grade string
	}{
		{95, "A"}, {85, "B"}, {75, "C"}, {65, "D"}, {45, "F"},
	}
	for _, tt := range tests {
		got := scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestEmptyTextScores(t *testing.T) {
	score, _ := scoreResponse("")
	if score != 0 {
		t.Fatalf("expected 0 for empty text, got %.1f", score)
	}
}

func TestScoreBounds(t *testing.T) {
	texts := []string{
		"yes",
		"However, it depends, and it might be contested. Some argue differently. The evidence is uncertain, mixed, and inconclusive. Perhaps this is debated with ongoing research from multiple viewpoints and different schools of thought. Others believe alternatively.",
		"Always. Never. Certainly definitely absolutely guaranteed without exception 100% obviously clearly undoubtedly impossible.",
	}
	for _, text := range texts {
		score, _ := scoreResponse(text)
		if score < 0 || score > 100 {
			t.Fatalf("score %.1f out of bounds for text: %s", score, text[:20])
		}
	}
}

func TestHighLowCounts(t *testing.T) {
	fs := []model.Finding{
		{
			ID: "f1", Lab: "lab-1",
			Summary: "However, it depends. Some argue X, others believe Y. The debate is uncertain with mixed evidence from multiple viewpoints and different schools of thought. Perhaps there are caveats, although not always clear.",
			Model:   model.ModelRef{Model: "m1"},
		},
		{
			ID: "f2", Lab: "lab-2",
			Summary: "Always true. Certainly guaranteed without exception. Obviously.",
			Model:   model.ModelRef{Model: "m1"},
		},
	}
	r := Analyze(fs)
	total := r.Models[0].HighNuance + r.Models[0].LowNuance
	if total < 1 {
		t.Fatal("expected at least 1 high or low nuance count")
	}
}

func TestFormatText(t *testing.T) {
	fs := mkFindings("However, this depends. Perhaps in some cases it might work.")
	r := Analyze(fs)
	text := FormatText(r)
	if !strings.Contains(text, "Nuance Score") {
		t.Fatal("expected header in output")
	}
	if !strings.Contains(text, "test-model") {
		t.Fatal("expected model name in output")
	}
}

func TestFormatTextEmpty(t *testing.T) {
	r := Analyze(nil)
	text := FormatText(r)
	if !strings.Contains(text, "No nuance data") {
		t.Fatal("expected empty message")
	}
}

func TestFormatJSON(t *testing.T) {
	fs := mkFindings("However, this is debated.")
	r := Analyze(fs)
	out := FormatJSON(r)
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := data["models"]; !ok {
		t.Fatal("expected models key in JSON")
	}
}

func TestUnknownModel(t *testing.T) {
	fs := []model.Finding{
		{ID: "f1", Lab: "lab-1", Summary: "test", Model: model.ModelRef{}},
	}
	r := Analyze(fs)
	if r.Models[0].Model != "unknown" {
		t.Fatalf("expected unknown, got %s", r.Models[0].Model)
	}
}

func TestLabCounting(t *testing.T) {
	fs := []model.Finding{
		{ID: "f1", Lab: "lab-1", Summary: "a", Model: model.ModelRef{Model: "m1"}},
		{ID: "f2", Lab: "lab-2", Summary: "b", Model: model.ModelRef{Model: "m1"}},
		{ID: "f3", Lab: "lab-1", Summary: "c", Model: model.ModelRef{Model: "m1"}},
	}
	r := Analyze(fs)
	if r.Models[0].LabCount != 2 {
		t.Fatalf("expected 2 labs, got %d", r.Models[0].LabCount)
	}
}
