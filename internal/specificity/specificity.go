package specificity

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var vagueTerms = map[string]bool{
	"something":  true,
	"somehow":    true,
	"somewhere":  true,
	"various":    true,
	"several":    true,
	"some":       true,
	"many":       true,
	"certain":    true,
	"things":     true,
	"stuff":      true,
	"issue":      true,
	"issues":     true,
	"problem":    true,
	"problems":   true,
	"might":      true,
	"maybe":      true,
	"possibly":   true,
	"perhaps":    true,
	"probably":   true,
	"apparently": true,
	"generally":  true,
	"typically":  true,
	"usually":    true,
	"kind":       true,
	"sort":       true,
	"quite":      true,
	"fairly":     true,
	"rather":     true,
	"somewhat":   true,
}

var specificIndicators = []string{
	"cve-", "cwe-", "owasp", "cvss",
	"line", "file", "function", "method", "class", "endpoint",
	"parameter", "variable", "column", "table", "field",
	"port", "protocol", "header", "cookie",
	"version", "v1", "v2", "v3",
	"http", "https", "tcp", "udp",
	"sql", "xss", "csrf", "ssrf", "rce", "lfi", "rfi",
	"injection", "overflow", "traversal", "deserialization",
}

type SpecificityDetail struct {
	VaguenessScore    float64 `json:"vagueness_score"`
	TechnicalDepth    float64 `json:"technical_depth"`
	LocationPrecision float64 `json:"location_precision"`
	EvidenceQuality   float64 `json:"evidence_quality"`
}

type ModelSpecificity struct {
	Model           string            `json:"model"`
	SpecificityScore float64          `json:"specificity_score"`
	Grade           string            `json:"grade"`
	Detail          SpecificityDetail `json:"detail"`
	SampleCount     int               `json:"sample_count"`
	VagueCount      int               `json:"vague_count"`
	SpecificCount   int               `json:"specific_count"`
}

type Report struct {
	Models         []ModelSpecificity `json:"models"`
	MostSpecific   string             `json:"most_specific"`
	LeastSpecific  string             `json:"least_specific"`
	MeanScore      float64            `json:"mean_score"`
}

func vaguenessScore(summary string) float64 {
	words := strings.Fields(strings.ToLower(summary))
	if len(words) == 0 {
		return 0
	}

	vagueCount := 0
	for _, w := range words {
		clean := strings.Trim(w, ".,;:!?\"'()[]{}")
		if vagueTerms[clean] {
			vagueCount++
		}
	}

	ratio := float64(vagueCount) / float64(len(words))
	return math.Round(math.Max(0, 100-ratio*500) * 10) / 10
}

func technicalDepth(summary string) float64 {
	lower := strings.ToLower(summary)
	matches := 0
	for _, ind := range specificIndicators {
		if strings.Contains(lower, ind) {
			matches++
		}
	}

	score := math.Min(100, float64(matches)/5*100)
	return math.Round(score*10) / 10
}

func locationPrecision(summary string) float64 {
	lower := strings.ToLower(summary)
	score := 30.0

	locationPhrases := []string{
		"line ", "file ", "in function ", "in method ", "at endpoint ",
		"in class ", "on port ", "in parameter ", "in column ",
		"in table ", ".py:", ".js:", ".go:", ".java:", ".rb:",
		"src/", "lib/", "app/", "api/",
	}

	for _, lp := range locationPhrases {
		if strings.Contains(lower, lp) {
			score += 15
		}
	}

	if score > 100 {
		score = 100
	}
	return math.Round(score*10) / 10
}

func evidenceQuality(lab, summary string) float64 {
	score := 40.0
	lower := strings.ToLower(summary)

	evidenceMarkers := []string{
		"because", "caused by", "due to", "as a result",
		"example", "for instance", "specifically",
		"payload", "request", "response", "input",
		"output", "stack trace", "error message",
		"proof", "demonstrates", "reproduc",
	}

	for _, m := range evidenceMarkers {
		if strings.Contains(lower, m) {
			score += 8
		}
	}

	labWords := extractKeywords(lab)
	sumWords := extractKeywords(summary)
	if len(labWords) > 0 {
		overlap := 0
		for w := range labWords {
			if sumWords[w] > 0 {
				overlap++
			}
		}
		if float64(overlap)/float64(len(labWords)) > 0.5 {
			score += 10
		}
	}

	if score > 100 {
		score = 100
	}
	return math.Round(score*10) / 10
}

var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"to": true, "of": true, "in": true, "for": true, "on": true,
	"with": true, "at": true, "by": true, "from": true, "as": true,
	"and": true, "but": true, "or": true, "not": true,
	"it": true, "its": true, "this": true, "that": true,
}

func extractKeywords(text string) map[string]int {
	words := strings.Fields(strings.ToLower(text))
	kw := map[string]int{}
	for _, w := range words {
		clean := strings.Trim(w, ".,;:!?\"'()[]{}")
		if len(clean) < 2 || stopWords[clean] {
			continue
		}
		kw[clean]++
	}
	return kw
}

func isVague(summary string) bool {
	return vaguenessScore(summary) < 50
}

func isSpecific(summary string) bool {
	return technicalDepth(summary) >= 60
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

	type pair struct {
		lab     string
		summary string
	}
	modelPairs := map[string][]pair{}
	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		modelPairs[mn] = append(modelPairs[mn], pair{lab: f.Lab, summary: f.Summary})
	}

	var models []ModelSpecificity
	for mn, pairs := range modelPairs {
		var sumVague, sumTech, sumLoc, sumEvid float64
		vagueCount := 0
		specificCount := 0

		for _, p := range pairs {
			v := vaguenessScore(p.summary)
			t := technicalDepth(p.summary)
			l := locationPrecision(p.summary)
			e := evidenceQuality(p.lab, p.summary)

			sumVague += v
			sumTech += t
			sumLoc += l
			sumEvid += e

			if isVague(p.summary) {
				vagueCount++
			}
			if isSpecific(p.summary) {
				specificCount++
			}
		}

		n := float64(len(pairs))
		avgV := sumVague / n
		avgT := sumTech / n
		avgL := sumLoc / n
		avgE := sumEvid / n

		overall := avgV*0.25 + avgT*0.30 + avgL*0.20 + avgE*0.25

		models = append(models, ModelSpecificity{
			Model:           mn,
			SpecificityScore: math.Round(overall*10) / 10,
			Grade:           scoreToGrade(overall),
			Detail: SpecificityDetail{
				VaguenessScore:    math.Round(avgV*10) / 10,
				TechnicalDepth:    math.Round(avgT*10) / 10,
				LocationPrecision: math.Round(avgL*10) / 10,
				EvidenceQuality:   math.Round(avgE*10) / 10,
			},
			SampleCount:   len(pairs),
			VagueCount:    vagueCount,
			SpecificCount: specificCount,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].SpecificityScore > models[j].SpecificityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.SpecificityScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostSpecific = models[0].Model
		r.LeastSpecific = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Specificity Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No specificity data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:    %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:          %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most specific:       %s\n", r.MostSpecific)
	fmt.Fprintf(&b, "  Least specific:      %s\n\n", r.LeastSpecific)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples)\n",
			m.Model, m.Grade, m.SpecificityScore, m.SampleCount)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "vagueness_score", m.Detail.VaguenessScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "technical_depth", m.Detail.TechnicalDepth)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "location_precision", m.Detail.LocationPrecision)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "evidence_quality", m.Detail.EvidenceQuality)
		fmt.Fprintf(&b, "    Vague answers: %d  Specific: %d\n",
			m.VagueCount, m.SpecificCount)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
