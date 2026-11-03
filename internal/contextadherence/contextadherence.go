package contextadherence

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type AdherenceDetail struct {
	ContextKeywordsTotal   int     `json:"context_keywords_total"`
	ContextKeywordsUsed    int     `json:"context_keywords_used"`
	KeywordCoverage        float64 `json:"keyword_coverage"`
	HallucinatedClaims     int     `json:"hallucinated_claims"`
	ContextDriftScore      float64 `json:"context_drift_score"`
	SelectiveUsageScore    float64 `json:"selective_usage_score"`
}

type ModelAdherence struct {
	Model           string          `json:"model"`
	AdherenceScore  float64         `json:"adherence_score"`
	Grade           string          `json:"grade"`
	Detail          AdherenceDetail `json:"detail"`
	SampleCount     int             `json:"sample_count"`
	LabCount        int             `json:"lab_count"`
	HighAdherence   int             `json:"high_adherence"`
	LowAdherence    int             `json:"low_adherence"`
}

type Report struct {
	Models          []ModelAdherence `json:"models"`
	MostAdherent    string           `json:"most_adherent"`
	LeastAdherent   string           `json:"least_adherent"`
	MeanScore       float64          `json:"mean_score"`
}

var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "to": true,
	"of": true, "in": true, "for": true, "on": true, "with": true,
	"at": true, "by": true, "from": true, "and": true, "or": true,
	"it": true, "this": true, "that": true, "i": true, "you": true,
	"we": true, "they": true, "do": true, "does": true, "did": true,
	"not": true, "but": true, "so": true, "if": true, "as": true,
	"what": true, "how": true, "why": true, "when": true, "where": true,
	"which": true, "who": true, "would": true, "could": true, "should": true,
	"can": true, "will": true, "has": true, "had": true, "have": true,
	"its": true, "their": true, "your": true, "my": true, "our": true,
	"about": true, "more": true, "also": true, "than": true, "then": true,
}

var hallucinationIndicators = []string{
	"as everyone knows", "it is well known that", "studies show",
	"research has proven", "experts agree", "according to",
	"statistics indicate", "data suggests", "historically",
	"in fact", "actually", "interestingly",
}

func extractKeywords(text string) map[string]int {
	words := strings.Fields(strings.ToLower(text))
	kw := map[string]int{}
	for _, w := range words {
		clean := strings.Trim(w, ".,;:!?\"'()[]{}")
		if len(clean) > 2 && !stopWords[clean] {
			kw[clean]++
		}
	}
	return kw
}

func keywordCoverage(context, response string) (total, used int, coverage float64) {
	ctxKW := extractKeywords(context)
	respKW := extractKeywords(response)
	total = len(ctxKW)
	if total == 0 {
		return 0, 0, 1.0
	}
	for kw := range ctxKW {
		if respKW[kw] > 0 {
			used++
		}
	}
	coverage = float64(used) / float64(total)
	return
}

func detectHallucinations(context, response string) int {
	lowerCtx := strings.ToLower(context)
	lowerResp := strings.ToLower(response)

	count := 0
	for _, ind := range hallucinationIndicators {
		if strings.Contains(lowerResp, ind) && !strings.Contains(lowerCtx, ind) {
			count++
		}
	}

	respKW := extractKeywords(response)
	ctxKW := extractKeywords(context)
	novelTerms := 0
	for kw, c := range respKW {
		if c >= 2 && ctxKW[kw] == 0 && len(kw) > 4 {
			novelTerms++
		}
	}
	if novelTerms > 5 {
		count += novelTerms / 5
	}

	return count
}

func contextDrift(context, response string) float64 {
	ctxKW := extractKeywords(context)
	respKW := extractKeywords(response)

	if len(respKW) == 0 {
		return 0
	}

	inContext := 0
	outOfContext := 0
	for kw := range respKW {
		if ctxKW[kw] > 0 {
			inContext++
		} else if len(kw) > 3 {
			outOfContext++
		}
	}

	total := inContext + outOfContext
	if total == 0 {
		return 0
	}
	return float64(outOfContext) / float64(total)
}

func selectiveUsage(context, response string) float64 {
	ctxKW := extractKeywords(context)
	if len(ctxKW) == 0 {
		return 0
	}

	type kwFreq struct {
		word  string
		count int
	}
	var sorted []kwFreq
	for w, c := range ctxKW {
		sorted = append(sorted, kwFreq{w, c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	respKW := extractKeywords(response)
	topN := len(sorted) / 3
	if topN < 1 {
		topN = 1
	}
	bottomN := topN

	topUsed := 0
	for i := 0; i < topN && i < len(sorted); i++ {
		if respKW[sorted[i].word] > 0 {
			topUsed++
		}
	}

	bottomUsed := 0
	for i := len(sorted) - bottomN; i < len(sorted); i++ {
		if i >= 0 && respKW[sorted[i].word] > 0 {
			bottomUsed++
		}
	}

	topRatio := float64(topUsed) / float64(topN)
	bottomRatio := float64(bottomUsed) / float64(bottomN)

	bias := topRatio - bottomRatio
	if bias < 0 {
		bias = 0
	}
	return bias
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

func ScoreResponse(context, response string) (float64, AdherenceDetail) {
	total, used, coverage := keywordCoverage(context, response)
	hallucinations := detectHallucinations(context, response)
	drift := contextDrift(context, response)
	selective := selectiveUsage(context, response)

	detail := AdherenceDetail{
		ContextKeywordsTotal: total,
		ContextKeywordsUsed:  used,
		KeywordCoverage:      math.Round(coverage*1000) / 1000,
		HallucinatedClaims:   hallucinations,
		ContextDriftScore:    math.Round(drift*1000) / 1000,
		SelectiveUsageScore:  math.Round(selective*1000) / 1000,
	}

	coverageScore := coverage * 40
	hallucinationPenalty := float64(hallucinations) * 8
	driftPenalty := drift * 30
	selectivePenalty := selective * 10

	score := 50 + coverageScore - hallucinationPenalty - driftPenalty - selectivePenalty
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	return math.Round(score*10) / 10, detail
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

	var models []ModelAdherence
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail AdherenceDetail
		highAdherence := 0
		lowAdherence := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			context := e.Location.PathTemplate
			if context == "" {
				context = e.Lab
			}
			score, detail := ScoreResponse(context, e.Summary)
			totalScore += score
			totalDetail.ContextKeywordsTotal += detail.ContextKeywordsTotal
			totalDetail.ContextKeywordsUsed += detail.ContextKeywordsUsed
			totalDetail.HallucinatedClaims += detail.HallucinatedClaims

			if score >= 75 {
				highAdherence++
			} else if score < 40 {
				lowAdherence++
			}
		}

		avgScore := totalScore / float64(len(entries))
		if totalDetail.ContextKeywordsTotal > 0 {
			totalDetail.KeywordCoverage = math.Round(
				float64(totalDetail.ContextKeywordsUsed)/float64(totalDetail.ContextKeywordsTotal)*1000) / 1000
		}

		models = append(models, ModelAdherence{
			Model:          mn,
			AdherenceScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			HighAdherence:  highAdherence,
			LowAdherence:   lowAdherence,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].AdherenceScore > models[j].AdherenceScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.AdherenceScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostAdherent = models[0].Model
		r.LeastAdherent = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Context Adherence Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No adherence data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most adherent:        %s\n", r.MostAdherent)
	fmt.Fprintf(&b, "  Least adherent:       %s\n\n", r.LeastAdherent)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.AdherenceScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %d / %d\n", "context_keywords",
			m.Detail.ContextKeywordsUsed, m.Detail.ContextKeywordsTotal)
		fmt.Fprintf(&b, "    %-28s %.3f\n", "keyword_coverage", m.Detail.KeywordCoverage)
		fmt.Fprintf(&b, "    %-28s %d\n", "hallucinated_claims", m.Detail.HallucinatedClaims)
		fmt.Fprintf(&b, "    %-28s %.3f\n", "context_drift", m.Detail.ContextDriftScore)
		fmt.Fprintf(&b, "    %-28s %.3f\n", "selective_usage", m.Detail.SelectiveUsageScore)
		fmt.Fprintf(&b, "    High adherence: %d  Low adherence: %d\n", m.HighAdherence, m.LowAdherence)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
