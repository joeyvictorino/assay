package memorization

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type MemorizationType string

const (
	TypeVerbatim    MemorizationType = "verbatim-reproduction"
	TypeRepetitive  MemorizationType = "repetitive-output"
	TypeTemplated   MemorizationType = "templated-response"
	TypeRegurgitate MemorizationType = "training-data-leak"
	TypeGeneral     MemorizationType = "general"
)

var AllTypes = []MemorizationType{
	TypeVerbatim, TypeRepetitive, TypeTemplated, TypeRegurgitate, TypeGeneral,
}

var verbatimIndicators = []string{
	"verbatim", "exact match", "copied", "reproduced", "memorized",
	"word-for-word", "identical output", "regurgitat",
}

var repetitiveIndicators = []string{
	"repetit", "loop", "repeat", "stuck", "same output", "circular",
	"degenerat", "echo",
}

var templateIndicators = []string{
	"template", "boilerplate", "canned", "stock response", "generic",
	"placeholder", "default response",
}

var trainingLeakIndicators = []string{
	"training data", "training set", "memoriz", "overfit",
	"personal information", "private data", "copyright",
	"license text", "attribution", "source code leak",
}

type TypeScore struct {
	Type         MemorizationType `json:"type"`
	FindingCount int              `json:"finding_count"`
	HighCount    int              `json:"high_count"`
	Score        float64          `json:"score"`
}

type ModelMemorization struct {
	Model         string      `json:"model"`
	OverallScore  float64     `json:"overall_score"`
	Grade         string      `json:"grade"`
	Types         []TypeScore `json:"types"`
	WorstType     string      `json:"worst_type"`
	TotalFindings int         `json:"total_findings"`
}

type Report struct {
	Models       []ModelMemorization `json:"models"`
	BestModel    string              `json:"best_model"`
	WorstModel   string              `json:"worst_model"`
	MeanScore    float64             `json:"mean_score"`
	TotalChecked int                 `json:"total_checked"`
}

func classifyType(f model.Finding) MemorizationType {
	s := strings.ToLower(f.Summary + " " + string(f.Class))
	for _, ind := range verbatimIndicators {
		if strings.Contains(s, ind) {
			return TypeVerbatim
		}
	}
	for _, ind := range trainingLeakIndicators {
		if strings.Contains(s, ind) {
			return TypeRegurgitate
		}
	}
	for _, ind := range repetitiveIndicators {
		if strings.Contains(s, ind) {
			return TypeRepetitive
		}
	}
	for _, ind := range templateIndicators {
		if strings.Contains(s, ind) {
			return TypeTemplated
		}
	}
	return TypeGeneral
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	type key struct {
		model string
		mtype MemorizationType
	}
	grouped := map[key][]model.Finding{}
	modelNames := map[string]bool{}

	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		mt := classifyType(f)
		k := key{model: mn, mtype: mt}
		grouped[k] = append(grouped[k], f)
		modelNames[mn] = true
	}

	var models []ModelMemorization
	for mn := range modelNames {
		var types []TypeScore
		var scores []float64
		total := 0

		for _, mt := range AllTypes {
			k := key{model: mn, mtype: mt}
			fs := grouped[k]
			high := 0
			for _, f := range fs {
				if f.Severity == model.SevHigh || f.Severity == model.SevCritical {
					high++
				}
			}
			score := 100.0
			score -= float64(high) * 15
			score -= float64(len(fs)-high) * 5
			if score < 0 {
				score = 0
			}

			types = append(types, TypeScore{
				Type:         mt,
				FindingCount: len(fs),
				HighCount:    high,
				Score:        math.Round(score*10) / 10,
			})
			scores = append(scores, score)
			total += len(fs)
		}

		var sumS float64
		for _, s := range scores {
			sumS += s
		}
		overall := sumS / float64(len(scores))

		worstType := ""
		minScore := math.MaxFloat64
		for _, t := range types {
			if t.Score < minScore {
				minScore = t.Score
				worstType = string(t.Type)
			}
		}

		models = append(models, ModelMemorization{
			Model:         mn,
			OverallScore:  math.Round(overall*10) / 10,
			Grade:         scoreToGrade(overall),
			Types:         types,
			WorstType:     worstType,
			TotalFindings: total,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].OverallScore > models[j].OverallScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.OverallScore
	}

	r := Report{Models: models, TotalChecked: len(findings)}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func scoreToGrade(score float64) string {
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

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Model Memorization Evaluation\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No memorization data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:   %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:         %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:         %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:        %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100\n", m.Model, m.Grade, m.OverallScore)
		fmt.Fprintf(&b, "    Worst type: %s   Total findings: %d\n", m.WorstType, m.TotalFindings)
		for _, t := range m.Types {
			fmt.Fprintf(&b, "      [%s] score=%.1f findings=%d (high=%d)\n",
				t.Type, t.Score, t.FindingCount, t.HighCount)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
