package edgecase

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type EdgeCaseDetail struct {
	BoundaryScore    float64 `json:"boundary_score"`
	NullHandling     float64 `json:"null_handling"`
	OverflowHandling float64 `json:"overflow_handling"`
	EmptyInputScore  float64 `json:"empty_input_score"`
	AmbiguityScore   float64 `json:"ambiguity_score"`
	BoundaryCount    int     `json:"boundary_count"`
	NullCount        int     `json:"null_count"`
	OverflowCount    int     `json:"overflow_count"`
}

type ModelEdgeCase struct {
	Model         string         `json:"model"`
	EdgeCaseScore float64        `json:"edge_case_score"`
	Grade         string         `json:"grade"`
	Detail        EdgeCaseDetail `json:"detail"`
	SampleCount   int            `json:"sample_count"`
	Concerns      int            `json:"concerns"`
}

type Report struct {
	Models     []ModelEdgeCase `json:"models"`
	BestModel  string          `json:"best_model"`
	WorstModel string          `json:"worst_model"`
	MeanScore  float64         `json:"mean_score"`
}

var boundaryPhrases = []string{
	"edge case", "boundary", "corner case", "limit",
	"extreme value", "off-by-one", "fence post",
	"boundary condition", "threshold", "borderline",
	"maximum", "minimum", "upper bound", "lower bound",
}

var nullPhrases = []string{
	"null", "nil", "none", "empty", "undefined",
	"missing value", "absent", "not provided",
	"zero-length", "no input", "blank",
	"unset", "not specified", "default value",
}

var overflowPhrases = []string{
	"overflow", "underflow", "wraparound", "truncation",
	"exceeds", "too large", "too small", "out of range",
	"integer overflow", "buffer overflow", "stack overflow",
	"capacity exceeded", "size limit", "length limit",
}

var ambiguityPhrases = []string{
	"ambiguous", "unclear", "vague", "multiple interpretations",
	"could mean", "depends on context", "it depends",
	"not well-defined", "unspecified behavior", "implementation-defined",
	"undefined behavior", "platform-dependent", "may vary",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, EdgeCaseDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))
	boundary := countPhrases(combined, boundaryPhrases)
	null := countPhrases(combined, nullPhrases)
	overflow := countPhrases(combined, overflowPhrases)
	ambiguity := countPhrases(combined, ambiguityPhrases)

	boundaryScore := 0.3
	if words > 0 {
		boundaryScore = math.Min(1.0, float64(boundary)*2.0/float64(words)*10.0+0.3)
	}
	nullScore := 0.3
	if null > 0 {
		nullScore = math.Min(1.0, float64(null)*0.12+0.3)
	}
	overflowScore := 0.3
	if overflow > 0 {
		overflowScore = math.Min(1.0, float64(overflow)*0.12+0.3)
	}
	emptyScore := 0.4
	if boundary > 0 || null > 0 {
		emptyScore = math.Min(1.0, emptyScore+float64(boundary+null)*0.08)
	}
	ambiguityScore := 0.3
	if ambiguity > 0 {
		ambiguityScore = math.Min(1.0, float64(ambiguity)*0.15+0.3)
	}

	detail := EdgeCaseDetail{
		BoundaryScore:    math.Round(boundaryScore*100) / 100,
		NullHandling:     math.Round(nullScore*100) / 100,
		OverflowHandling: math.Round(overflowScore*100) / 100,
		EmptyInputScore:  math.Round(emptyScore*100) / 100,
		AmbiguityScore:   math.Round(ambiguityScore*100) / 100,
		BoundaryCount:    boundary,
		NullCount:        null,
		OverflowCount:    overflow,
	}

	score := 20.0
	score += boundaryScore * 20.0
	score += nullScore * 15.0
	score += overflowScore * 15.0
	score += emptyScore * 15.0
	score += ambiguityScore * 10.0
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10, detail
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

func evidenceText(f model.Finding) string {
	var parts []string
	for _, e := range f.Evidence {
		if e.Marker != "" {
			parts = append(parts, e.Marker)
		}
	}
	if len(parts) == 0 {
		return f.Summary
	}
	return strings.Join(parts, " ")
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}
	modelEntries := map[string][]model.Finding{}
	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		modelEntries[mn] = append(modelEntries[mn], f)
	}
	var models []ModelEdgeCase
	for mn, ff := range modelEntries {
		var total float64
		var concerns int
		var detail EdgeCaseDetail
		for _, f := range ff {
			s, d := scoreResponse(f.Summary, evidenceText(f))
			total += s
			detail.BoundaryCount += d.BoundaryCount
			detail.NullCount += d.NullCount
			detail.OverflowCount += d.OverflowCount
			detail.BoundaryScore += d.BoundaryScore
			detail.NullHandling += d.NullHandling
			detail.OverflowHandling += d.OverflowHandling
			detail.EmptyInputScore += d.EmptyInputScore
			detail.AmbiguityScore += d.AmbiguityScore
			if s < 50 {
				concerns++
			}
		}
		n := float64(len(ff))
		avg := math.Round(total/n*10) / 10
		detail.BoundaryScore = math.Round(detail.BoundaryScore/n*100) / 100
		detail.NullHandling = math.Round(detail.NullHandling/n*100) / 100
		detail.OverflowHandling = math.Round(detail.OverflowHandling/n*100) / 100
		detail.EmptyInputScore = math.Round(detail.EmptyInputScore/n*100) / 100
		detail.AmbiguityScore = math.Round(detail.AmbiguityScore/n*100) / 100
		models = append(models, ModelEdgeCase{
			Model: mn, EdgeCaseScore: avg, Grade: scoreToGrade(avg),
			Detail: detail, SampleCount: len(ff), Concerns: concerns,
		})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].EdgeCaseScore > models[j].EdgeCaseScore })
	var sum float64
	for _, m := range models {
		sum += m.EdgeCaseScore
	}
	r := Report{Models: models, MeanScore: math.Round(sum/float64(len(models))*10) / 10}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Edge Case Handling Accuracy (%d models, mean %.1f)\n\n", len(r.Models), r.MeanScore)
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %-30s  score: %5.1f  grade: %s  samples: %d  concerns: %d\n",
			m.Model, m.EdgeCaseScore, m.Grade, m.SampleCount, m.Concerns)
		fmt.Fprintf(&b, "    boundary=%.2f  null=%.2f  overflow=%.2f  empty=%.2f  ambiguity=%.2f\n",
			m.Detail.BoundaryScore, m.Detail.NullHandling, m.Detail.OverflowHandling,
			m.Detail.EmptyInputScore, m.Detail.AmbiguityScore)
	}
	if r.BestModel != "" {
		fmt.Fprintf(&b, "\n  Best:  %s\n  Worst: %s\n", r.BestModel, r.WorstModel)
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
