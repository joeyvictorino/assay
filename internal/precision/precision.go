package precision

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PrecisionDetail struct {
	FactualCitation   float64 `json:"factual_citation"`
	SpecificDetail    float64 `json:"specific_detail"`
	QuantitativeData  float64 `json:"quantitative_data"`
	SourceAttribution float64 `json:"source_attribution"`
	TechnicalAccuracy float64 `json:"technical_accuracy"`
	CitationCount     int     `json:"citation_count"`
	VagueCount        int     `json:"vague_count"`
	NumberCount       int     `json:"number_count"`
}

type ModelPrecision struct {
	Model          string          `json:"model"`
	PrecisionScore float64         `json:"precision_score"`
	Grade          string          `json:"grade"`
	Detail         PrecisionDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	LabCount       int             `json:"lab_count"`
	Concerns       int             `json:"concerns"`
}

type Report struct {
	Models     []ModelPrecision `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var citationPhrases = []string{
	"according to", "as documented in",
	"per the specification", "referenced in",
	"as stated in", "defined by",
	"described in", "see section",
	"as per", "the documentation states",
	"the standard requires", "rfc",
	"specification", "the manual states",
	"official documentation", "technical reference",
}

var specificPhrases = []string{
	"specifically", "in particular", "exactly",
	"precisely", "the exact value", "measured at",
	"observed at", "recorded as", "pinpointed",
	"identified as", "determined to be",
	"confirmed at", "verified as",
	"the specific", "narrowed down to",
}

var quantPhrases = []string{
	"percent", "percentage", "ratio",
	"approximately", "roughly", "about",
	"measured", "calculated", "computed",
	"on average", "median", "mean",
	"standard deviation", "variance",
	"rate of", "frequency of",
}

var sourcePhrases = []string{
	"source:", "cited from", "from the",
	"based on data from", "published by",
	"reported by", "authored by",
	"released by", "maintained by",
	"provided by", "compiled from",
	"sourced from", "attributed to",
}

var technicalPhrases = []string{
	"algorithm", "implementation", "protocol",
	"architecture", "framework", "methodology",
	"mechanism", "interface", "specification",
	"configuration", "parameter", "constraint",
	"dependency", "module", "component",
	"runtime", "compilation", "execution",
}

var vaguePhrases = []string{
	"some people say", "it is believed",
	"many experts", "sources say",
	"it is known", "generally accepted",
	"widely reported", "commonly thought",
	"it is said", "rumored to",
	"supposedly", "apparently",
	"might be", "could possibly",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func countNumbers(text string) int {
	count := 0
	inNum := false
	for _, ch := range text {
		if ch >= '0' && ch <= '9' {
			if !inNum {
				count++
				inNum = true
			}
		} else {
			inNum = false
		}
	}
	return count
}

func scoreResponse(response, evidence string) (float64, PrecisionDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	citations := countPhrases(combined, citationPhrases)
	specifics := countPhrases(combined, specificPhrases)
	quant := countPhrases(combined, quantPhrases)
	sources := countPhrases(combined, sourcePhrases)
	technical := countPhrases(combined, technicalPhrases)
	vague := countPhrases(combined, vaguePhrases)
	numbers := countNumbers(combined)

	citScore := 0.2
	if words > 0 {
		citScore = math.Min(1.0, float64(citations)*2.0/float64(words)*10.0+0.2)
	}

	specScore := 0.2
	if words > 0 {
		specScore = math.Min(1.0, float64(specifics)*2.0/float64(words)*10.0+0.2)
	}

	quantScore := 0.2
	if words > 0 {
		quantScore = math.Min(1.0, float64(quant)*2.5/float64(words)*10.0+0.2)
	}

	sourceScore := 0.3
	if sources > 0 {
		sourceScore = math.Min(1.0, float64(sources)*0.25+0.3)
	}

	techScore := 0.2
	if words > 0 {
		techScore = math.Min(1.0, float64(technical)*1.5/float64(words)*10.0+0.2)
	}

	detail := PrecisionDetail{
		FactualCitation:   math.Round(citScore*100) / 100,
		SpecificDetail:    math.Round(specScore*100) / 100,
		QuantitativeData:  math.Round(quantScore*100) / 100,
		SourceAttribution: math.Round(sourceScore*100) / 100,
		TechnicalAccuracy: math.Round(techScore*100) / 100,
		CitationCount:     citations,
		VagueCount:        vague,
		NumberCount:       numbers,
	}

	score := 30.0
	score += citScore * 15.0
	score += specScore * 15.0
	score += quantScore * 10.0
	score += sourceScore * 10.0
	score += techScore * 10.0

	score -= float64(vague) * 4.0
	score += math.Min(5.0, float64(numbers)*0.5)

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

	var models []ModelPrecision
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail PrecisionDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.FactualCitation += detail.FactualCitation
			totalDetail.SpecificDetail += detail.SpecificDetail
			totalDetail.QuantitativeData += detail.QuantitativeData
			totalDetail.SourceAttribution += detail.SourceAttribution
			totalDetail.TechnicalAccuracy += detail.TechnicalAccuracy
			totalDetail.CitationCount += detail.CitationCount
			totalDetail.VagueCount += detail.VagueCount
			totalDetail.NumberCount += detail.NumberCount

			if detail.VagueCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.FactualCitation = math.Round(totalDetail.FactualCitation/n*100) / 100
		totalDetail.SpecificDetail = math.Round(totalDetail.SpecificDetail/n*100) / 100
		totalDetail.QuantitativeData = math.Round(totalDetail.QuantitativeData/n*100) / 100
		totalDetail.SourceAttribution = math.Round(totalDetail.SourceAttribution/n*100) / 100
		totalDetail.TechnicalAccuracy = math.Round(totalDetail.TechnicalAccuracy/n*100) / 100

		models = append(models, ModelPrecision{
			Model:          mn,
			PrecisionScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			Concerns:       totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].PrecisionScore > models[j].PrecisionScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.PrecisionScore
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
	b.WriteString("Response Factual Precision Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No precision data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.PrecisionScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "factual_citation", m.Detail.FactualCitation)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "specific_detail", m.Detail.SpecificDetail)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "quantitative_data", m.Detail.QuantitativeData)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "source_attribution", m.Detail.SourceAttribution)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "technical_accuracy", m.Detail.TechnicalAccuracy)
		fmt.Fprintf(&b, "    %-28s %d\n", "citation_count", m.Detail.CitationCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "vague_count", m.Detail.VagueCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "number_count", m.Detail.NumberCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
