package instructfollow

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type DimensionScore struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
}

type ModelResult struct {
	Model        string           `json:"model"`
	OverallScore float64          `json:"overall_score"`
	Grade        string           `json:"grade"`
	Dimensions   []DimensionScore `json:"dimensions"`
	SampleCount  int              `json:"sample_count"`
}

type Report struct {
	Models      []ModelResult `json:"models"`
	BestModel   string        `json:"best_model"`
	WorstModel  string        `json:"worst_model"`
	MeanScore   float64       `json:"mean_score"`
}

func gradeFromScore(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

func constraintAdherence(findings []model.Finding) float64 {
	if len(findings) == 0 {
		return 50
	}
	var scores []float64
	for _, f := range findings {
		summary := strings.ToLower(f.Summary)
		score := 80.0
		if strings.Contains(summary, "ignor") || strings.Contains(summary, "miss") || strings.Contains(summary, "omit") {
			score -= 30
		}
		if strings.Contains(summary, "contradict") || strings.Contains(summary, "violat") {
			score -= 40
		}
		if strings.Contains(summary, "partial") {
			score -= 15
		}
		if score < 0 {
			score = 0
		}
		scores = append(scores, score)
	}
	var sum float64
	for _, s := range scores {
		sum += s
	}
	return math.Round(sum/float64(len(scores))*10) / 10
}

func formatCompliance(findings []model.Finding) float64 {
	if len(findings) == 0 {
		return 50
	}
	var scores []float64
	for _, f := range findings {
		summary := strings.ToLower(f.Summary)
		score := 85.0
		if strings.Contains(summary, "format") && (strings.Contains(summary, "wrong") || strings.Contains(summary, "incorrect")) {
			score -= 35
		}
		if strings.Contains(summary, "json") || strings.Contains(summary, "xml") || strings.Contains(summary, "markdown") {
			if strings.Contains(summary, "invalid") || strings.Contains(summary, "malform") {
				score -= 40
			}
		}
		if score < 0 {
			score = 0
		}
		scores = append(scores, score)
	}
	var sum float64
	for _, s := range scores {
		sum += s
	}
	return math.Round(sum/float64(len(scores))*10) / 10
}

func taskCompletion(findings []model.Finding) float64 {
	if len(findings) == 0 {
		return 50
	}
	var scores []float64
	for _, f := range findings {
		summary := strings.ToLower(f.Summary)
		score := 75.0
		if strings.Contains(summary, "refus") || strings.Contains(summary, "decline") {
			score -= 45
		} else if strings.Contains(summary, "incomplete") || strings.Contains(summary, "truncat") {
			score -= 30
		} else if strings.Contains(summary, "complete") || strings.Contains(summary, "fulfill") {
			score += 15
		}
		if score < 0 {
			score = 0
		}
		if score > 100 {
			score = 100
		}
		scores = append(scores, score)
	}
	var sum float64
	for _, s := range scores {
		sum += s
	}
	return math.Round(sum/float64(len(scores))*10) / 10
}

func Analyze(findings []model.Finding) Report {
	byModel := map[string][]model.Finding{}
	for _, f := range findings {
		key := f.Model.Model
		if key == "" {
			key = "unknown"
		}
		byModel[key] = append(byModel[key], f)
	}

	var models []ModelResult
	for name, mf := range byModel {
		dims := []DimensionScore{
			{Name: "constraint_adherence", Score: constraintAdherence(mf)},
			{Name: "format_compliance", Score: formatCompliance(mf)},
			{Name: "task_completion", Score: taskCompletion(mf)},
		}
		var total float64
		for _, d := range dims {
			total += d.Score
		}
		overall := math.Round(total/float64(len(dims))*10) / 10
		models = append(models, ModelResult{
			Model:        name,
			OverallScore: overall,
			Grade:        gradeFromScore(overall),
			Dimensions:   dims,
			SampleCount:  len(mf),
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].OverallScore > models[j].OverallScore
	})

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		var sum float64
		for _, m := range models {
			sum += m.OverallScore
		}
		r.MeanScore = math.Round(sum/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Instruction Following Analysis (%d models)\n\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:  %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:  %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model: %s\n\n", r.WorstModel)
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %-25s  %5.1f (%s)  n=%d\n", m.Model, m.OverallScore, m.Grade, m.SampleCount)
		for _, d := range m.Dimensions {
			fmt.Fprintf(&b, "    %-23s %5.1f\n", d.Name, d.Score)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
