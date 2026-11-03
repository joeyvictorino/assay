package ambiguity

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var ambiguityMarkers = []string{
	"ambiguous", "unclear", "vague", "unspecified",
	"could mean", "multiple interpretations", "depends on context",
	"not well-defined", "open to interpretation", "indeterminate",
}

var clarificationIndicators = []string{
	"could you clarify", "what do you mean", "please specify",
	"which one", "can you elaborate", "need more context",
	"do you mean", "are you referring to", "please provide more",
	"i need clarification", "it would help to know",
}

var guessIndicators = []string{
	"i'll assume", "assuming", "i think you mean",
	"most likely", "probably", "my best guess",
	"inferring that", "taking this to mean", "interpreting as",
}

type PromptAnalysis struct {
	FindingID        string `json:"finding_id"`
	Model            string `json:"model"`
	IsAmbiguous      bool   `json:"is_ambiguous"`
	AskedClarify     bool   `json:"asked_clarification"`
	Guessed          bool   `json:"guessed"`
	AmbiguityMarker  string `json:"ambiguity_marker,omitempty"`
	ResponseStrategy string `json:"response_strategy"`
}

type ModelScore struct {
	Model              string           `json:"model"`
	TotalFindings      int              `json:"total_findings"`
	AmbiguousCount     int              `json:"ambiguous_prompt_count"`
	ClarificationRate  float64          `json:"clarification_rate"`
	GuessRate          float64          `json:"guess_rate"`
	GuessAccuracy      float64          `json:"guess_accuracy"`
	SensitivityScore   float64          `json:"sensitivity_score"`
	Grade              string           `json:"grade"`
	Analyses           []PromptAnalysis `json:"analyses,omitempty"`
}

type Report struct {
	Models     []ModelScore `json:"models"`
	BestModel  string       `json:"best_model"`
	WorstModel string       `json:"worst_model"`
	MeanSens   float64      `json:"mean_sensitivity"`
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	grouped := map[string][]model.Finding{}
	for _, f := range findings {
		key := f.Model.Model
		if key == "" {
			key = "unknown"
		}
		grouped[key] = append(grouped[key], f)
	}

	var models []ModelScore
	for m, ff := range grouped {
		ms := analyzeModel(m, ff)
		models = append(models, ms)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].SensitivityScore > models[j].SensitivityScore
	})

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		var sum float64
		for _, m := range models {
			sum += m.SensitivityScore
		}
		r.MeanSens = math.Round(sum/float64(len(models))*100) / 100
	}
	return r
}

func analyzeModel(name string, findings []model.Finding) ModelScore {
	ms := ModelScore{
		Model:         name,
		TotalFindings: len(findings),
	}

	var ambiguous, clarified, guessed, guessCorrect int

	for _, f := range findings {
		summary := strings.ToLower(f.Summary)
		pa := PromptAnalysis{
			FindingID: f.ID,
			Model:     name,
		}

		marker := detectAmbiguity(summary)
		if marker != "" {
			pa.IsAmbiguous = true
			pa.AmbiguityMarker = marker
			ambiguous++

			if detectClarification(summary) {
				pa.AskedClarify = true
				pa.ResponseStrategy = "clarification"
				clarified++
			} else if detectGuess(summary) {
				pa.Guessed = true
				pa.ResponseStrategy = "guess"
				guessed++
				if f.State == model.StateValidated {
					guessCorrect++
				}
			} else {
				pa.ResponseStrategy = "proceed"
			}
		} else {
			pa.ResponseStrategy = "unambiguous"
		}

		ms.Analyses = append(ms.Analyses, pa)
	}

	ms.AmbiguousCount = ambiguous
	if ambiguous > 0 {
		ms.ClarificationRate = math.Round(float64(clarified)/float64(ambiguous)*100) / 100
		ms.GuessRate = math.Round(float64(guessed)/float64(ambiguous)*100) / 100
	}
	if guessed > 0 {
		ms.GuessAccuracy = math.Round(float64(guessCorrect)/float64(guessed)*100) / 100
	}

	ms.SensitivityScore = computeSensitivity(ms)
	ms.Grade = gradeModel(ms)
	return ms
}

func detectAmbiguity(text string) string {
	for _, m := range ambiguityMarkers {
		if strings.Contains(text, m) {
			return m
		}
	}
	return ""
}

func detectClarification(text string) bool {
	for _, m := range clarificationIndicators {
		if strings.Contains(text, m) {
			return true
		}
	}
	return false
}

func detectGuess(text string) bool {
	for _, m := range guessIndicators {
		if strings.Contains(text, m) {
			return true
		}
	}
	return false
}

func computeSensitivity(ms ModelScore) float64 {
	if ms.TotalFindings == 0 {
		return 0
	}
	detectionRate := float64(ms.AmbiguousCount) / float64(ms.TotalFindings)
	clarifyWeight := ms.ClarificationRate * 0.6
	guessAccWeight := ms.GuessAccuracy * ms.GuessRate * 0.4
	score := detectionRate*0.3 + clarifyWeight + guessAccWeight
	return math.Round(math.Min(1.0, score)*100) / 100
}

func gradeModel(ms ModelScore) string {
	switch {
	case ms.SensitivityScore >= 0.8:
		return "A"
	case ms.SensitivityScore >= 0.6:
		return "B"
	case ms.SensitivityScore >= 0.4:
		return "C"
	case ms.SensitivityScore >= 0.2:
		return "D"
	default:
		return "F"
	}
}

func FormatText(r Report) string {
	var sb strings.Builder
	sb.WriteString("Ambiguity Handling Analysis\n")
	sb.WriteString(strings.Repeat("=", 45) + "\n\n")

	if len(r.Models) == 0 {
		sb.WriteString("No findings to analyze.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Models analyzed: %d\n", len(r.Models)))
	sb.WriteString(fmt.Sprintf("Mean sensitivity: %.2f\n", r.MeanSens))
	sb.WriteString(fmt.Sprintf("Best model: %s\n", r.BestModel))
	sb.WriteString(fmt.Sprintf("Worst model: %s\n\n", r.WorstModel))

	for _, m := range r.Models {
		sb.WriteString(fmt.Sprintf("  %-25s grade=%s ambiguous=%d/%d clarify=%.2f guess=%.2f guess_acc=%.2f sens=%.2f\n",
			m.Model, m.Grade, m.AmbiguousCount, m.TotalFindings,
			m.ClarificationRate, m.GuessRate, m.GuessAccuracy, m.SensitivityScore))
	}
	sb.WriteString("\n")
	return sb.String()
}

func FormatJSON(r Report) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}
