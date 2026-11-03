package thoroughness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ThoroughnessDetail struct {
	CoverageScore    float64 `json:"coverage_score"`
	DetailScore      float64 `json:"detail_score"`
	MethodScore      float64 `json:"method_score"`
	VerificationScore float64 `json:"verification_score"`
	EdgeCaseScore    float64 `json:"edge_case_score"`
	WordCount        int     `json:"word_count"`
	CoverageTerms   int     `json:"coverage_terms"`
	GapTerms        int     `json:"gap_terms"`
}

type ModelThoroughness struct {
	Model             string              `json:"model"`
	ThoroughnessScore float64             `json:"thoroughness_score"`
	Grade             string              `json:"grade"`
	Detail            ThoroughnessDetail  `json:"detail"`
	SampleCount       int                 `json:"sample_count"`
	LabCount          int                 `json:"lab_count"`
	Concerns          int                 `json:"concerns"`
}

type Report struct {
	Models     []ModelThoroughness `json:"models"`
	BestModel  string              `json:"best_model"`
	WorstModel string              `json:"worst_model"`
	MeanScore  float64             `json:"mean_score"`
}

var coverageIndicators = []string{
	"comprehensively", "thoroughly", "exhaustively",
	"in full", "covering all", "across all",
	"each aspect", "every dimension", "all factors",
	"complete analysis", "full examination", "end to end",
	"from start to finish", "all relevant", "nothing overlooked",
	"all angles", "every perspective",
}

var detailIndicators = []string{
	"specifically", "in detail", "precisely",
	"the exact", "granular", "fine-grained",
	"step by step", "broken down", "decomposed",
	"at the level of", "drilling into", "examining closely",
	"in depth", "nuanced", "subtlety",
	"the specifics", "particularities",
}

var methodIndicators = []string{
	"methodology", "systematic", "framework",
	"approach", "procedure", "protocol",
	"technique", "strategy", "algorithm",
	"heuristic", "following the", "applying the",
	"using the method", "structured approach", "rigorous",
}

var verificationIndicators = []string{
	"verified", "confirmed", "validated",
	"cross-checked", "double-checked", "corroborated",
	"tested against", "compared with", "benchmarked",
	"audited", "reviewed by", "independently confirmed",
	"replicated", "peer-reviewed", "fact-checked",
}

var edgeCaseIndicators = []string{
	"edge case", "corner case", "boundary condition",
	"special case", "exception", "unusual scenario",
	"worst case", "best case", "degenerate case",
	"when empty", "when null", "when zero",
	"overflow", "underflow", "timeout",
	"race condition", "concurrency", "deadlock",
}

var gapIndicators = []string{
	"not covered", "omitted", "skipped",
	"left out", "missing", "incomplete",
	"partial", "superficial", "cursory",
	"glossed over", "hand-waved", "vague",
	"unaddressed", "ignored", "overlooked",
}

func countIndicators(text string, indicators []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, ind := range indicators {
		count += strings.Count(lower, ind)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, ThoroughnessDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	coverage := countIndicators(combined, coverageIndicators)
	detail := countIndicators(combined, detailIndicators)
	method := countIndicators(combined, methodIndicators)
	verification := countIndicators(combined, verificationIndicators)
	edgeCase := countIndicators(combined, edgeCaseIndicators)
	gaps := countIndicators(combined, gapIndicators)

	coverageScore := math.Min(1.0, float64(coverage)*0.12+0.25)
	if words > 80 {
		coverageScore = math.Min(1.0, coverageScore+0.15)
	} else if words > 40 {
		coverageScore = math.Min(1.0, coverageScore+0.08)
	}

	detailScore := math.Min(1.0, float64(detail)*0.11+0.25)
	if words > 60 {
		detailScore = math.Min(1.0, detailScore+0.1)
	}

	methodScore := math.Min(1.0, float64(method)*0.13+0.2)

	verificationScore := math.Min(1.0, float64(verification)*0.14+0.15)

	edgeCaseScore := math.Min(1.0, float64(edgeCase)*0.15+0.1)

	gapPenalty := math.Min(1.0, float64(gaps)*0.1)
	coverageScore = math.Max(0, coverageScore-gapPenalty*0.5)
	detailScore = math.Max(0, detailScore-gapPenalty*0.3)

	td := ThoroughnessDetail{
		CoverageScore:     math.Round(coverageScore*100) / 100,
		DetailScore:       math.Round(detailScore*100) / 100,
		MethodScore:       math.Round(methodScore*100) / 100,
		VerificationScore: math.Round(verificationScore*100) / 100,
		EdgeCaseScore:     math.Round(edgeCaseScore*100) / 100,
		WordCount:         words,
		CoverageTerms:     coverage,
		GapTerms:          gaps,
	}

	score := 15.0
	score += coverageScore * 20.0
	score += detailScore * 18.0
	score += methodScore * 16.0
	score += verificationScore * 15.0
	score += edgeCaseScore * 14.0
	score -= gapPenalty * 20.0

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10, td
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

	var models []ModelThoroughness
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail ThoroughnessDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.CoverageScore += detail.CoverageScore
			totalDetail.DetailScore += detail.DetailScore
			totalDetail.MethodScore += detail.MethodScore
			totalDetail.VerificationScore += detail.VerificationScore
			totalDetail.EdgeCaseScore += detail.EdgeCaseScore
			totalDetail.WordCount += detail.WordCount
			totalDetail.CoverageTerms += detail.CoverageTerms
			totalDetail.GapTerms += detail.GapTerms

			if detail.CoverageScore < 0.35 || detail.DetailScore < 0.35 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.CoverageScore = math.Round(totalDetail.CoverageScore/n*100) / 100
		totalDetail.DetailScore = math.Round(totalDetail.DetailScore/n*100) / 100
		totalDetail.MethodScore = math.Round(totalDetail.MethodScore/n*100) / 100
		totalDetail.VerificationScore = math.Round(totalDetail.VerificationScore/n*100) / 100
		totalDetail.EdgeCaseScore = math.Round(totalDetail.EdgeCaseScore/n*100) / 100

		models = append(models, ModelThoroughness{
			Model:             mn,
			ThoroughnessScore: math.Round(avgScore*10) / 10,
			Grade:             scoreToGrade(avgScore),
			Detail:            totalDetail,
			SampleCount:       len(entries),
			LabCount:          len(labs),
			Concerns:          totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ThoroughnessScore > models[j].ThoroughnessScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ThoroughnessScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Thoroughness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No thoroughness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.ThoroughnessScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "coverage_score", m.Detail.CoverageScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "detail_score", m.Detail.DetailScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "method_score", m.Detail.MethodScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "verification_score", m.Detail.VerificationScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "edge_case_score", m.Detail.EdgeCaseScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "word_count", m.Detail.WordCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "coverage_terms", m.Detail.CoverageTerms)
		fmt.Fprintf(&b, "    %-28s %d\n", "gap_terms", m.Detail.GapTerms)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
