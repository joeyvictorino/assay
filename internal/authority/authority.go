package authority

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type AuthorityDetail struct {
	DomainKnowledge   float64 `json:"domain_knowledge"`
	ConfidenceMarkers float64 `json:"confidence_markers"`
	SourceAttribution float64 `json:"source_attribution"`
	ExpertiseSignals  float64 `json:"expertise_signals"`
	PrecisionLang     float64 `json:"precision_lang"`
	SourceCount       int     `json:"source_count"`
	HedgeCount        int     `json:"hedge_count"`
	TechnicalCount    int     `json:"technical_count"`
}

type ModelAuthority struct {
	Model          string          `json:"model"`
	AuthorityScore float64         `json:"authority_score"`
	Grade          string          `json:"grade"`
	Detail         AuthorityDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	LabCount       int             `json:"lab_count"`
	Concerns       int             `json:"concerns"`
}

type Report struct {
	Models     []ModelAuthority `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var domainPhrases = []string{
	"technically", "architecturally", "fundamentally",
	"the specification", "per the standard",
	"according to the documentation", "the protocol",
	"the algorithm", "the implementation",
	"in computer science", "in software engineering",
	"the data structure", "the design pattern",
	"the framework", "the runtime",
}

var confidencePhrases = []string{
	"the correct approach", "the recommended way",
	"the standard practice", "definitively",
	"this is because", "the reason is",
	"this ensures", "this guarantees",
	"the key insight", "the important thing",
	"critically", "essentially",
	"the solution is", "the answer is",
}

var sourcePhrases = []string{
	"according to", "as documented in",
	"the official documentation", "the specification states",
	"rfc", "per the standard",
	"the source code shows", "the api reference",
	"research indicates", "studies show",
	"the documentation says", "as noted in",
	"the manual states", "reference implementation",
}

var expertisePhrases = []string{
	"best practice", "industry standard",
	"production-grade", "enterprise-ready",
	"battle-tested", "well-established",
	"proven approach", "mature solution",
	"widely adopted", "commonly used in",
	"professional-grade", "recommended by",
}

var precisionPhrases = []string{
	"specifically", "precisely", "exactly",
	"in particular", "namely", "i.e.",
	"e.g.", "to be precise", "concretely",
	"the exact", "the specific",
	"measured at", "quantified as",
}

var uncertainPhrases = []string{
	"i'm not sure", "i think", "maybe",
	"possibly", "i believe", "it might be",
	"i'm uncertain", "i don't know",
	"i could be wrong", "don't quote me",
	"take this with a grain", "roughly",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, AuthorityDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	domain := countPhrases(combined, domainPhrases)
	confidence := countPhrases(combined, confidencePhrases)
	sources := countPhrases(combined, sourcePhrases)
	expertise := countPhrases(combined, expertisePhrases)
	precision := countPhrases(combined, precisionPhrases)
	uncertain := countPhrases(combined, uncertainPhrases)

	domainScore := 0.2
	if words > 0 {
		domainScore = math.Min(1.0, float64(domain)*2.0/float64(words)*10.0+0.2)
	}

	confScore := 0.2
	if words > 0 {
		confScore = math.Min(1.0, float64(confidence)*2.0/float64(words)*10.0+0.2)
	}

	srcScore := 0.2
	if sources > 0 {
		srcScore = math.Min(1.0, float64(sources)*0.2+0.2)
	}

	expScore := 0.3
	if expertise > 0 {
		expScore = math.Min(1.0, float64(expertise)*0.25+0.3)
	}

	precScore := 0.3
	if precision > 0 {
		precScore = math.Min(1.0, float64(precision)*0.2+0.3)
	}

	technical := domain + expertise

	detail := AuthorityDetail{
		DomainKnowledge:   math.Round(domainScore*100) / 100,
		ConfidenceMarkers: math.Round(confScore*100) / 100,
		SourceAttribution: math.Round(srcScore*100) / 100,
		ExpertiseSignals:  math.Round(expScore*100) / 100,
		PrecisionLang:     math.Round(precScore*100) / 100,
		SourceCount:       sources,
		HedgeCount:        uncertain,
		TechnicalCount:    technical,
	}

	score := 30.0
	score += domainScore * 15.0
	score += confScore * 15.0
	score += srcScore * 15.0
	score += expScore * 10.0
	score += precScore * 10.0

	score -= float64(uncertain) * 4.0

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

	var models []ModelAuthority
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail AuthorityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.DomainKnowledge += detail.DomainKnowledge
			totalDetail.ConfidenceMarkers += detail.ConfidenceMarkers
			totalDetail.SourceAttribution += detail.SourceAttribution
			totalDetail.ExpertiseSignals += detail.ExpertiseSignals
			totalDetail.PrecisionLang += detail.PrecisionLang
			totalDetail.SourceCount += detail.SourceCount
			totalDetail.HedgeCount += detail.HedgeCount
			totalDetail.TechnicalCount += detail.TechnicalCount

			if detail.HedgeCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.DomainKnowledge = math.Round(totalDetail.DomainKnowledge/n*100) / 100
		totalDetail.ConfidenceMarkers = math.Round(totalDetail.ConfidenceMarkers/n*100) / 100
		totalDetail.SourceAttribution = math.Round(totalDetail.SourceAttribution/n*100) / 100
		totalDetail.ExpertiseSignals = math.Round(totalDetail.ExpertiseSignals/n*100) / 100
		totalDetail.PrecisionLang = math.Round(totalDetail.PrecisionLang/n*100) / 100

		models = append(models, ModelAuthority{
			Model:          mn,
			AuthorityScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			Concerns:       totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].AuthorityScore > models[j].AuthorityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.AuthorityScore
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
	b.WriteString("Response Authority and Expertise Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No authority data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.AuthorityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "domain_knowledge", m.Detail.DomainKnowledge)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "confidence_markers", m.Detail.ConfidenceMarkers)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "source_attribution", m.Detail.SourceAttribution)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "expertise_signals", m.Detail.ExpertiseSignals)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "precision_lang", m.Detail.PrecisionLang)
		fmt.Fprintf(&b, "    %-28s %d\n", "source_count", m.Detail.SourceCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "hedge_count", m.Detail.HedgeCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "technical_count", m.Detail.TechnicalCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
