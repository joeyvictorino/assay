package idempotency

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type IdempotencyDetail struct {
	TotalPairs       int     `json:"total_pairs"`
	ConsistentPairs  int     `json:"consistent_pairs"`
	InconsistentPairs int    `json:"inconsistent_pairs"`
	ConsistencyRatio float64 `json:"consistency_ratio"`
	SeverityFlips    int     `json:"severity_flips"`
	ClassFlips       int     `json:"class_flips"`
	StateFlips       int     `json:"state_flips"`
}

type ModelIdempotency struct {
	Model            string            `json:"model"`
	IdempotencyScore float64           `json:"idempotency_score"`
	Grade            string            `json:"grade"`
	Detail           IdempotencyDetail `json:"detail"`
	SampleCount      int               `json:"sample_count"`
	LabCount         int               `json:"lab_count"`
}

type Report struct {
	Models          []ModelIdempotency `json:"models"`
	MostConsistent  string             `json:"most_consistent"`
	LeastConsistent string             `json:"least_consistent"`
	MeanScore       float64            `json:"mean_score"`
}

func grade(score float64) string {
	switch {
	case score >= 0.95:
		return "A"
	case score >= 0.85:
		return "B"
	case score >= 0.70:
		return "C"
	case score >= 0.50:
		return "D"
	default:
		return "F"
	}
}

type findingKey struct {
	lab      string
	location string
}

func keyOf(f model.Finding) findingKey {
	return findingKey{
		lab:      f.Lab,
		location: f.Location.Method + " " + f.Location.PathTemplate,
	}
}

func Analyze(findings []model.Finding) Report {
	byModel := map[string][]model.Finding{}
	for _, f := range findings {
		key := f.Model.Model
		if key == "" {
			key = f.Model.Provider + "/" + f.Model.Agent
		}
		byModel[key] = append(byModel[key], f)
	}

	var models []ModelIdempotency
	for name, ff := range byModel {
		m := analyzeModel(name, ff)
		models = append(models, m)
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].IdempotencyScore > models[j].IdempotencyScore
	})

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostConsistent = models[0].Model
		r.LeastConsistent = models[len(models)-1].Model
		sum := 0.0
		for _, m := range models {
			sum += m.IdempotencyScore
		}
		r.MeanScore = sum / float64(len(models))
	}
	return r
}

func analyzeModel(name string, findings []model.Finding) ModelIdempotency {
	labs := map[string]bool{}
	grouped := map[findingKey][]model.Finding{}
	for _, f := range findings {
		labs[f.Lab] = true
		k := keyOf(f)
		grouped[k] = append(grouped[k], f)
	}

	detail := IdempotencyDetail{}
	for _, group := range grouped {
		if len(group) < 2 {
			continue
		}
		for i := 0; i < len(group)-1; i++ {
			for j := i + 1; j < len(group); j++ {
				detail.TotalPairs++
				consistent := true
				if group[i].Severity != group[j].Severity {
					detail.SeverityFlips++
					consistent = false
				}
				if group[i].Class != group[j].Class {
					detail.ClassFlips++
					consistent = false
				}
				if group[i].State != group[j].State {
					detail.StateFlips++
					consistent = false
				}
				if consistent {
					detail.ConsistentPairs++
				} else {
					detail.InconsistentPairs++
				}
			}
		}
	}

	if detail.TotalPairs > 0 {
		detail.ConsistencyRatio = float64(detail.ConsistentPairs) / float64(detail.TotalPairs)
	} else {
		detail.ConsistencyRatio = 1.0
	}

	score := detail.ConsistencyRatio
	flipPenalty := 0.0
	if detail.TotalPairs > 0 {
		flipPenalty = float64(detail.SeverityFlips) * 0.1 / float64(detail.TotalPairs)
	}
	score = math.Max(0, score-flipPenalty)

	return ModelIdempotency{
		Model:            name,
		IdempotencyScore: math.Round(score*100) / 100,
		Grade:            grade(score),
		Detail:           detail,
		SampleCount:      len(findings),
		LabCount:         len(labs),
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Idempotency Analysis (%d models)\n\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:       %.2f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most consistent:  %s\n", r.MostConsistent)
	fmt.Fprintf(&b, "  Least consistent: %s\n\n", r.LeastConsistent)
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %-30s [%s] score=%.2f  pairs=%d  consistent=%d  flips(sev=%d cls=%d state=%d)\n",
			m.Model, m.Grade, m.IdempotencyScore,
			m.Detail.TotalPairs, m.Detail.ConsistentPairs,
			m.Detail.SeverityFlips, m.Detail.ClassFlips, m.Detail.StateFlips)
	}
	return b.String()
}

func FormatJSON(r Report) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b) + "\n"
}
