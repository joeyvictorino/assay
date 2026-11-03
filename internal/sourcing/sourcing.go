package sourcing

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type SourcingDetail struct {
	CitationDensity    float64 `json:"citation_density"`
	SourceVariety      float64 `json:"source_variety"`
	AttributionClarity float64 `json:"attribution_clarity"`
	ReferenceAccuracy  float64 `json:"reference_accuracy"`
	Traceability       float64 `json:"traceability"`
	CitedCount         int     `json:"cited_count"`
	UncitedCount       int     `json:"uncited_count"`
	VagueCount         int     `json:"vague_count"`
}

type ModelSourcing struct {
	Model        string         `json:"model"`
	SourcingScore float64       `json:"sourcing_score"`
	Grade        string         `json:"grade"`
	Detail       SourcingDetail `json:"detail"`
	SampleCount  int            `json:"sample_count"`
	LabCount     int            `json:"lab_count"`
	Concerns     int            `json:"concerns"`
}

type Report struct {
	Models     []ModelSourcing `json:"models"`
	BestModel  string          `json:"best_model"`
	WorstModel string          `json:"worst_model"`
	MeanScore  float64         `json:"mean_score"`
}

var citationPhrases = []string{
	"according to", "as stated in", "cited in",
	"referenced in", "as documented in", "per the",
	"source:", "reference:", "see also",
	"as noted by", "as reported by", "published in",
	"from the official", "in the documentation",
	"the specification states",
}

var varietyPhrases = []string{
	"research paper", "academic study", "peer-reviewed",
	"official documentation", "technical report", "rfc",
	"standard specification", "white paper", "case study",
	"blog post", "conference paper", "journal article",
	"government report", "industry report", "survey",
}

var attributionPhrases = []string{
	"authored by", "written by", "published by",
	"maintained by", "created by", "developed by",
	"version", "edition", "volume",
	"doi:", "isbn:", "issn:",
	"retrieved from", "accessed on", "available at",
}

var accuracyPhrases = []string{
	"verified against", "cross-referenced with",
	"confirmed by", "corroborated by",
	"consistent with", "in agreement with",
	"as validated by", "empirically shown",
	"reproducible", "independently verified",
	"fact-checked", "peer-reviewed",
}

var vagueSourcePhrases = []string{
	"some sources say", "it is widely believed",
	"many experts think", "studies show",
	"research suggests", "it is generally accepted",
	"common knowledge", "well known that",
	"people say", "they say",
	"i've heard", "supposedly",
}

var uncitedPhrases = []string{
	"i think", "i believe", "in my opinion",
	"i would say", "my guess is",
	"probably", "perhaps", "likely",
	"it seems", "it appears",
	"from what i understand", "as far as i know",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, SourcingDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	citations := countPhrases(combined, citationPhrases)
	variety := countPhrases(combined, varietyPhrases)
	attribution := countPhrases(combined, attributionPhrases)
	accuracy := countPhrases(combined, accuracyPhrases)
	vague := countPhrases(combined, vagueSourcePhrases)
	uncited := countPhrases(combined, uncitedPhrases)

	cited := citations + attribution

	densityScore := 0.2
	if words > 0 {
		densityScore = math.Min(1.0, float64(citations)*2.0/float64(words)*10.0+0.2)
	}

	varietyScore := 0.2
	if words > 0 {
		varietyScore = math.Min(1.0, float64(variety)*2.0/float64(words)*10.0+0.2)
	}

	attributionScore := 0.3
	if attribution > 0 || citations > 0 {
		attributionScore = math.Min(1.0, float64(attribution+citations)*0.12+0.3)
	}

	accuracyScore := 0.2
	if accuracy > 0 {
		accuracyScore = math.Min(1.0, float64(accuracy)*0.15+0.2)
	}

	traceScore := 0.2
	if vague == 0 && uncited == 0 {
		traceScore = 0.8
	} else if vague+uncited < 3 {
		traceScore = 0.5
	}

	detail := SourcingDetail{
		CitationDensity:    math.Round(densityScore*100) / 100,
		SourceVariety:      math.Round(varietyScore*100) / 100,
		AttributionClarity: math.Round(attributionScore*100) / 100,
		ReferenceAccuracy:  math.Round(accuracyScore*100) / 100,
		Traceability:       math.Round(traceScore*100) / 100,
		CitedCount:         cited,
		UncitedCount:       uncited,
		VagueCount:         vague,
	}

	score := 30.0
	score += densityScore * 15.0
	score += varietyScore * 15.0
	score += attributionScore * 10.0
	score += accuracyScore * 10.0
	score += traceScore * 10.0

	score -= float64(vague) * 4.0
	score -= float64(uncited) * 5.0

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

	var models []ModelSourcing
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail SourcingDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.CitationDensity += detail.CitationDensity
			totalDetail.SourceVariety += detail.SourceVariety
			totalDetail.AttributionClarity += detail.AttributionClarity
			totalDetail.ReferenceAccuracy += detail.ReferenceAccuracy
			totalDetail.Traceability += detail.Traceability
			totalDetail.CitedCount += detail.CitedCount
			totalDetail.UncitedCount += detail.UncitedCount
			totalDetail.VagueCount += detail.VagueCount

			if detail.UncitedCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.CitationDensity = math.Round(totalDetail.CitationDensity/n*100) / 100
		totalDetail.SourceVariety = math.Round(totalDetail.SourceVariety/n*100) / 100
		totalDetail.AttributionClarity = math.Round(totalDetail.AttributionClarity/n*100) / 100
		totalDetail.ReferenceAccuracy = math.Round(totalDetail.ReferenceAccuracy/n*100) / 100
		totalDetail.Traceability = math.Round(totalDetail.Traceability/n*100) / 100

		models = append(models, ModelSourcing{
			Model:        mn,
			SourcingScore: math.Round(avgScore*10) / 10,
			Grade:        scoreToGrade(avgScore),
			Detail:       totalDetail,
			SampleCount:  len(entries),
			LabCount:     len(labs),
			Concerns:     totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].SourcingScore > models[j].SourcingScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.SourcingScore
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
	b.WriteString("Source Attribution Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No sourcing data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.SourcingScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "citation_density", m.Detail.CitationDensity)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "source_variety", m.Detail.SourceVariety)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "attribution_clarity", m.Detail.AttributionClarity)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "reference_accuracy", m.Detail.ReferenceAccuracy)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "traceability", m.Detail.Traceability)
		fmt.Fprintf(&b, "    %-28s %d\n", "cited_count", m.Detail.CitedCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "uncited_count", m.Detail.UncitedCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "vague_count", m.Detail.VagueCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
