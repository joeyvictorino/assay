package selfcorrect

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var correctionMarkers = []string{
	"actually", "let me reconsider", "i was wrong", "correction:",
	"upon reflection", "i need to revise", "wait,", "sorry,",
	"let me correct", "on second thought", "i misspoke",
	"that's not right", "i should clarify", "more accurately",
	"to be more precise", "i retract", "disregard my earlier",
}

var overcorrectionMarkers = []string{
	"actually, my first answer was correct",
	"i was right the first time",
	"going back to my original",
	"my initial assessment was accurate",
	"reverting to my earlier answer",
}

type CorrectionEvent struct {
	FindingID  string `json:"finding_id"`
	Model      string `json:"model"`
	Marker     string `json:"marker"`
	Position   int    `json:"position"`
	SevBefore  string `json:"severity_before"`
	SevAfter   string `json:"severity_after"`
	Improved   bool   `json:"improved"`
	Overcorrect bool  `json:"overcorrect"`
}

type ModelScore struct {
	Model              string            `json:"model"`
	TotalFindings      int               `json:"total_findings"`
	CorrectionCount    int               `json:"correction_count"`
	CorrectionRate     float64           `json:"correction_rate"`
	AccuracyBefore     float64           `json:"accuracy_before"`
	AccuracyAfter      float64           `json:"accuracy_after"`
	ImprovementRate    float64           `json:"improvement_rate"`
	OvercorrectionRate float64           `json:"overcorrection_rate"`
	Grade              string            `json:"grade"`
	Events             []CorrectionEvent `json:"events,omitempty"`
}

type Report struct {
	Models     []ModelScore `json:"models"`
	BestModel  string       `json:"best_model"`
	WorstModel string       `json:"worst_model"`
	MeanRate   float64      `json:"mean_correction_rate"`
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
		return models[i].ImprovementRate > models[j].ImprovementRate
	})

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		var sum float64
		for _, m := range models {
			sum += m.CorrectionRate
		}
		r.MeanRate = math.Round(sum/float64(len(models))*100) / 100
	}
	return r
}

func analyzeModel(name string, findings []model.Finding) ModelScore {
	ms := ModelScore{
		Model:         name,
		TotalFindings: len(findings),
	}

	var corrections int
	var overcorrections int
	var improved int
	var validatedBefore, validatedAfter int

	for i, f := range findings {
		summary := strings.ToLower(f.Summary)
		marker := detectCorrection(summary)
		if marker == "" {
			if f.State == model.StateValidated {
				validatedBefore++
				validatedAfter++
			}
			continue
		}

		corrections++
		evt := CorrectionEvent{
			FindingID: f.ID,
			Model:     name,
			Marker:    marker,
			Position:  i,
			SevBefore: string(f.Severity),
		}

		if i > 0 {
			evt.SevBefore = string(findings[i-1].Severity)
		}
		evt.SevAfter = string(f.Severity)

		if f.State == model.StateValidated {
			validatedAfter++
			evt.Improved = true
			improved++
		}
		if i > 0 && findings[i-1].State == model.StateValidated {
			validatedBefore++
		}

		oc := detectOvercorrection(summary)
		if oc {
			overcorrections++
			evt.Overcorrect = true
		}

		ms.Events = append(ms.Events, evt)
	}

	ms.CorrectionCount = corrections
	if len(findings) > 0 {
		ms.CorrectionRate = math.Round(float64(corrections)/float64(len(findings))*100) / 100
	}
	if len(findings) > 0 {
		ms.AccuracyBefore = math.Round(float64(validatedBefore)/float64(len(findings))*100) / 100
		ms.AccuracyAfter = math.Round(float64(validatedAfter)/float64(len(findings))*100) / 100
	}
	if corrections > 0 {
		ms.ImprovementRate = math.Round(float64(improved)/float64(corrections)*100) / 100
		ms.OvercorrectionRate = math.Round(float64(overcorrections)/float64(corrections)*100) / 100
	}

	ms.Grade = gradeModel(ms)
	return ms
}

func detectCorrection(text string) string {
	for _, m := range correctionMarkers {
		if strings.Contains(text, m) {
			return m
		}
	}
	return ""
}

func detectOvercorrection(text string) bool {
	for _, m := range overcorrectionMarkers {
		if strings.Contains(text, m) {
			return true
		}
	}
	return false
}

func gradeModel(ms ModelScore) string {
	score := ms.ImprovementRate*0.5 + (1-ms.OvercorrectionRate)*0.3 + ms.AccuracyAfter*0.2
	switch {
	case score >= 0.9:
		return "A"
	case score >= 0.8:
		return "B"
	case score >= 0.6:
		return "C"
	case score >= 0.4:
		return "D"
	default:
		return "F"
	}
}

func FormatText(r Report) string {
	var sb strings.Builder
	sb.WriteString("Self-Correction Analysis\n")
	sb.WriteString(strings.Repeat("=", 45) + "\n\n")

	if len(r.Models) == 0 {
		sb.WriteString("No findings to analyze.\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Models analyzed: %d\n", len(r.Models)))
	sb.WriteString(fmt.Sprintf("Mean correction rate: %.2f\n", r.MeanRate))
	sb.WriteString(fmt.Sprintf("Best model: %s\n", r.BestModel))
	sb.WriteString(fmt.Sprintf("Worst model: %s\n\n", r.WorstModel))

	for _, m := range r.Models {
		sb.WriteString(fmt.Sprintf("  %-25s grade=%s corrections=%d/%d rate=%.2f improve=%.2f overcorrect=%.2f\n",
			m.Model, m.Grade, m.CorrectionCount, m.TotalFindings,
			m.CorrectionRate, m.ImprovementRate, m.OvercorrectionRate))
	}
	sb.WriteString("\n")
	return sb.String()
}

func FormatJSON(r Report) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}
