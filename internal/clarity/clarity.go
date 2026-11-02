package clarity

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Dimension string

const (
	DimAmbiguity    Dimension = "ambiguity"
	DimJargon       Dimension = "jargon"
	DimStructure    Dimension = "structure"
	DimSpecificity  Dimension = "specificity"
	DimGeneral      Dimension = "general"
)

var AllDimensions = []Dimension{
	DimAmbiguity, DimJargon, DimStructure, DimSpecificity, DimGeneral,
}

var ambiguityIndicators = []string{
	"it depends", "could be", "might be", "possibly",
	"perhaps", "arguably", "it's unclear", "ambiguous",
	"not sure", "hard to say",
}

var jargonIndicators = []string{
	"leverage", "synergy", "paradigm", "ontology",
	"epistemological", "heuristic", "modality", "tokenization",
	"hyperparameter", "backpropagation",
}

var structureIndicators = []string{
	"firstly", "secondly", "in summary", "to conclude",
	"step 1", "step 2", "bullet point", "numbered list",
	"here's a breakdown", "let me outline",
}

var specificityIndicators = []string{
	"specifically", "for example", "such as", "in particular",
	"concretely", "namely", "precisely", "exactly",
	"for instance", "to illustrate",
}

type DimensionScore struct {
	Dimension    Dimension `json:"dimension"`
	FindingCount int       `json:"finding_count"`
	HighCount    int       `json:"high_count"`
	Score        float64   `json:"score"`
}

type ModelClarity struct {
	Model          string           `json:"model"`
	OverallScore   float64          `json:"overall_score"`
	Grade          string           `json:"grade"`
	Dimensions     []DimensionScore `json:"dimensions"`
	ClarityIndex   float64          `json:"clarity_index"`
	TotalFindings  int              `json:"total_findings"`
}

type Report struct {
	Models     []ModelClarity `json:"models"`
	BestModel  string         `json:"best_model"`
	WorstModel string         `json:"worst_model"`
	MeanScore  float64        `json:"mean_score"`
}

func classifyDimension(f model.Finding) Dimension {
	s := strings.ToLower(f.Summary)
	for _, ind := range ambiguityIndicators {
		if strings.Contains(s, ind) {
			return DimAmbiguity
		}
	}
	for _, ind := range jargonIndicators {
		if strings.Contains(s, ind) {
			return DimJargon
		}
	}
	for _, ind := range structureIndicators {
		if strings.Contains(s, ind) {
			return DimStructure
		}
	}
	for _, ind := range specificityIndicators {
		if strings.Contains(s, ind) {
			return DimSpecificity
		}
	}
	return DimGeneral
}

func isHigh(f model.Finding) bool {
	return strings.EqualFold(string(f.Severity), "high") || strings.EqualFold(string(f.Severity), "critical")
}

func grade(score float64) string {
	switch {
	case score >= 0.9:
		return "A"
	case score >= 0.8:
		return "B"
	case score >= 0.7:
		return "C"
	case score >= 0.6:
		return "D"
	default:
		return "F"
	}
}

func Analyze(findings []model.Finding) Report {
	byModel := make(map[string][]model.Finding)
	for _, f := range findings {
		m := f.Model.Model
		if m == "" {
			m = "unknown"
		}
		byModel[m] = append(byModel[m], f)
	}

	var models []ModelClarity
	for m, ff := range byModel {
		ms := analyzeModel(m, ff)
		models = append(models, ms)
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
		r.MeanScore = math.Round(sum/float64(len(models))*1000) / 1000
	}
	return r
}

func analyzeModel(name string, findings []model.Finding) ModelClarity {
	dimCounts := make(map[Dimension]int)
	highCounts := make(map[Dimension]int)
	specific := 0

	for _, f := range findings {
		d := classifyDimension(f)
		dimCounts[d]++
		if isHigh(f) {
			highCounts[d]++
		}
		if d == DimSpecificity {
			specific++
		}
	}

	total := len(findings)
	var dimensions []DimensionScore

	for _, d := range AllDimensions {
		count := dimCounts[d]
		high := highCounts[d]
		score := 1.0
		if total > 0 {
			ratio := float64(count) / float64(total)
			highRatio := float64(high) / float64(maxInt(count, 1))
			score = math.Max(0, 1.0-ratio*0.5-highRatio*0.3)
			score = math.Round(score*1000) / 1000
		}
		dimensions = append(dimensions, DimensionScore{
			Dimension:    d,
			FindingCount: count,
			HighCount:    high,
			Score:        score,
		})
	}

	overall := 1.0
	if total > 0 {
		var sum float64
		for _, d := range dimensions {
			sum += d.Score
		}
		overall = math.Round(sum/float64(len(dimensions))*1000) / 1000
	}

	clarityIdx := 0.0
	if total > 0 {
		ambiguous := float64(dimCounts[DimAmbiguity] + dimCounts[DimJargon])
		clarityIdx = math.Round((1.0-ambiguous/float64(total))*1000) / 1000
	}

	return ModelClarity{
		Model:         name,
		OverallScore:  overall,
		Grade:         grade(overall),
		Dimensions:    dimensions,
		ClarityIndex:  clarityIdx,
		TotalFindings: total,
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Clarity Analysis (mean score: %.3f)\n\n", r.MeanScore)
	if r.BestModel != "" {
		fmt.Fprintf(&b, "  Best:  %s\n", r.BestModel)
		fmt.Fprintf(&b, "  Worst: %s\n\n", r.WorstModel)
	}
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %s [%s] score=%.3f findings=%d clarity_idx=%.3f\n",
			m.Model, m.Grade, m.OverallScore, m.TotalFindings, m.ClarityIndex)
		for _, d := range m.Dimensions {
			if d.FindingCount > 0 {
				fmt.Fprintf(&b, "    %-20s count=%d high=%d score=%.3f\n",
					d.Dimension, d.FindingCount, d.HighCount, d.Score)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
