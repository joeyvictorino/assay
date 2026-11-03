package rigor

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type RigorDetail struct {
	EvidenceCount   int     `json:"evidence_count"`
	LogicCount      int     `json:"logic_count"`
	QuantifierCount int     `json:"quantifier_count"`
	CitationCount   int     `json:"citation_count"`
	HandwaveCount   int     `json:"handwave_count"`
	WordCount       int     `json:"word_count"`
	EvidenceRatio   float64 `json:"evidence_ratio"`
}

type ModelRigor struct {
	Model       string      `json:"model"`
	Score       float64     `json:"score"`
	Grade       string      `json:"grade"`
	Detail      RigorDetail `json:"detail"`
	SampleCount int         `json:"sample_count"`
	LabCount    int         `json:"lab_count"`
}

type Report struct {
	Models     []ModelRigor `json:"models"`
	BestModel  string       `json:"best_model"`
	WorstModel string       `json:"worst_model"`
	MeanScore  float64      `json:"mean_score"`
}

var evidencePhrases = []string{
	"according to", "as shown by", "evidence suggests", "data indicates",
	"research shows", "studies confirm", "experiments demonstrate",
	"the results show", "measurements indicate", "analysis reveals",
	"logs confirm", "traces show", "the output demonstrates",
	"verified by", "confirmed by", "as documented in",
}

var logicPhrases = []string{
	"therefore", "consequently", "because", "since",
	"it follows that", "this implies", "given that",
	"as a result", "due to", "this means that",
	"if and only if", "necessarily", "by definition",
	"the reason is", "this is caused by",
}

var quantifierPhrases = []string{
	"exactly", "precisely", "approximately", "at least",
	"at most", "no more than", "no fewer than",
	"on average", "the median", "the mean",
	"per second", "per request", "per minute",
}

var citationPhrases = []string{
	"rfc ", "cve-", "owasp", "nist", "iso ",
	"section ", "specification", "standard",
	"documented at", "defined in", "per the",
}

var handwavePhrases = []string{
	"probably", "might be", "could be", "i think",
	"it seems", "sort of", "kind of", "somehow",
	"in general", "usually", "typically works",
	"should be fine", "most likely", "presumably",
	"i believe", "i assume", "i guess",
}

var numberPattern = regexp.MustCompile(`\b\d+(\.\d+)?\b`)

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func AnalyzeText(text string) (float64, RigorDetail) {
	words := strings.Fields(text)
	wordCount := len(words)

	if wordCount == 0 {
		return 50.0, RigorDetail{}
	}

	evidence := countPhrases(text, evidencePhrases)
	logic := countPhrases(text, logicPhrases)
	quantifiers := countPhrases(text, quantifierPhrases)
	citations := countPhrases(text, citationPhrases)
	handwaves := countPhrases(text, handwavePhrases)
	numbers := len(numberPattern.FindAllString(text, -1))

	positiveSignals := float64(evidence + logic + quantifiers + citations + numbers)
	negativeSignals := float64(handwaves)
	totalSignals := positiveSignals + negativeSignals

	evidenceRatio := 0.5
	if totalSignals > 0 {
		evidenceRatio = positiveSignals / totalSignals
	}

	score := 50.0
	score += float64(evidence) * 3.5
	score += float64(logic) * 3.0
	score += float64(quantifiers) * 2.5
	score += float64(citations) * 4.0
	score += float64(numbers) * 1.5
	score -= float64(handwaves) * 5.0

	density := positiveSignals / float64(wordCount) * 100
	score += density * 2.0

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	detail := RigorDetail{
		EvidenceCount:   evidence,
		LogicCount:      logic,
		QuantifierCount: quantifiers,
		CitationCount:   citations,
		HandwaveCount:   handwaves,
		WordCount:       wordCount,
		EvidenceRatio:   math.Round(evidenceRatio*100) / 100,
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

	var models []ModelRigor
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail RigorDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Summary)
			totalScore += score
			aggDetail.EvidenceCount += detail.EvidenceCount
			aggDetail.LogicCount += detail.LogicCount
			aggDetail.QuantifierCount += detail.QuantifierCount
			aggDetail.CitationCount += detail.CitationCount
			aggDetail.HandwaveCount += detail.HandwaveCount
			aggDetail.WordCount += detail.WordCount
			aggDetail.EvidenceRatio += detail.EvidenceRatio
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.EvidenceRatio = math.Round(aggDetail.EvidenceRatio/n*100) / 100

		models = append(models, ModelRigor{
			Model:       mn,
			Score:       math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      aggDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Score > models[j].Score
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.Score
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
	b.WriteString("Analytical Rigor Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No rigor data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Evidence: %d  Logic: %d  Quantifiers: %d\n",
			m.Detail.EvidenceCount, m.Detail.LogicCount, m.Detail.QuantifierCount)
		fmt.Fprintf(&b, "    Citations: %d  Handwaves: %d  Ratio: %.2f\n",
			m.Detail.CitationCount, m.Detail.HandwaveCount, m.Detail.EvidenceRatio)
		fmt.Fprintf(&b, "    Samples: %d  Labs: %d\n\n",
			m.SampleCount, m.LabCount)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
