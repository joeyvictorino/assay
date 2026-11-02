package relevance

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var stopWords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true, "need": true,
	"to": true, "of": true, "in": true, "for": true, "on": true,
	"with": true, "at": true, "by": true, "from": true, "as": true,
	"into": true, "through": true, "during": true, "before": true,
	"after": true, "above": true, "below": true, "between": true,
	"and": true, "but": true, "or": true, "nor": true, "not": true,
	"so": true, "yet": true, "both": true, "either": true, "neither": true,
	"this": true, "that": true, "these": true, "those": true,
	"it": true, "its": true, "i": true, "we": true, "you": true,
	"he": true, "she": true, "they": true, "me": true, "him": true,
	"her": true, "us": true, "them": true, "my": true, "your": true,
	"his": true, "our": true, "their": true,
}

type RelevanceDetail struct {
	KeywordOverlap  float64 `json:"keyword_overlap"`
	TopicAlignment  float64 `json:"topic_alignment"`
	DriftScore      float64 `json:"drift_score"`
	AnswerDirectness float64 `json:"answer_directness"`
}

type ModelRelevance struct {
	Model          string          `json:"model"`
	RelevanceScore float64         `json:"relevance_score"`
	Grade          string          `json:"grade"`
	Detail         RelevanceDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	DriftCount     int             `json:"drift_count"`
	OffTopicCount  int             `json:"off_topic_count"`
}

type Report struct {
	Models       []ModelRelevance `json:"models"`
	MostRelevant string           `json:"most_relevant"`
	LeastRelevant string          `json:"least_relevant"`
	MeanScore    float64          `json:"mean_score"`
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

func keywordOverlap(labKW, summaryKW map[string]int) float64 {
	if len(labKW) == 0 {
		return 50
	}
	matched := 0
	for k := range labKW {
		if summaryKW[k] > 0 {
			matched++
		}
	}
	return math.Min(100, float64(matched)/float64(len(labKW))*100)
}

func topicAlignment(lab, summary string) float64 {
	labWords := extractKeywords(lab)
	sumWords := extractKeywords(summary)
	if len(labWords) == 0 || len(sumWords) == 0 {
		return 50
	}

	labTotal := 0
	for _, c := range labWords {
		labTotal += c
	}
	sumTotal := 0
	for _, c := range sumWords {
		sumTotal += c
	}

	labVec := map[string]float64{}
	for w, c := range labWords {
		labVec[w] = float64(c) / float64(labTotal)
	}
	sumVec := map[string]float64{}
	for w, c := range sumWords {
		sumVec[w] = float64(c) / float64(sumTotal)
	}

	var dot, magA, magB float64
	allWords := map[string]bool{}
	for w := range labVec {
		allWords[w] = true
	}
	for w := range sumVec {
		allWords[w] = true
	}
	for w := range allWords {
		a := labVec[w]
		b := sumVec[w]
		dot += a * b
		magA += a * a
		magB += b * b
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	cosine := dot / (math.Sqrt(magA) * math.Sqrt(magB))
	return math.Round(cosine*1000) / 10
}

func driftScore(lab, summary string) float64 {
	labKW := extractKeywords(lab)
	sumKW := extractKeywords(summary)
	if len(sumKW) == 0 {
		return 100
	}
	offTopic := 0
	for w := range sumKW {
		found := false
		for lw := range labKW {
			if w == lw || strings.Contains(w, lw) || strings.Contains(lw, w) {
				found = true
				break
			}
		}
		if !found {
			offTopic++
		}
	}
	ratio := float64(offTopic) / float64(len(sumKW))
	score := 100 - math.Min(100, ratio*120)
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
}

func answerDirectness(summary string) float64 {
	lower := strings.ToLower(summary)
	hedgePhrases := []string{
		"it depends", "there are many", "various factors",
		"it's complicated", "generally speaking", "in general",
		"it varies", "there is no single", "there are several",
		"on the other hand", "however it",
	}
	hedgeCount := 0
	for _, h := range hedgePhrases {
		if strings.Contains(lower, h) {
			hedgeCount++
		}
	}

	words := strings.Fields(summary)
	if len(words) == 0 {
		return 50
	}

	score := 100 - math.Min(100, float64(hedgeCount)*15)
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10
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

	var models []ModelRelevance
	for mn, pairs := range modelPairs {
		var sumOverlap, sumTopic, sumDrift, sumDirect float64
		driftCount := 0
		offTopicCount := 0

		for _, p := range pairs {
			labKW := extractKeywords(p.lab)
			sumKW := extractKeywords(p.summary)

			overlap := keywordOverlap(labKW, sumKW)
			topic := topicAlignment(p.lab, p.summary)
			drift := driftScore(p.lab, p.summary)
			direct := answerDirectness(p.summary)

			sumOverlap += overlap
			sumTopic += topic
			sumDrift += drift
			sumDirect += direct

			if drift < 50 {
				driftCount++
			}
			if overlap < 20 {
				offTopicCount++
			}
		}

		n := float64(len(pairs))
		avgOverlap := sumOverlap / n
		avgTopic := sumTopic / n
		avgDrift := sumDrift / n
		avgDirect := sumDirect / n

		overall := (avgOverlap + avgTopic + avgDrift + avgDirect) / 4

		models = append(models, ModelRelevance{
			Model:          mn,
			RelevanceScore: math.Round(overall*10) / 10,
			Grade:          scoreToGrade(overall),
			Detail: RelevanceDetail{
				KeywordOverlap:  math.Round(avgOverlap*10) / 10,
				TopicAlignment:  math.Round(avgTopic*10) / 10,
				DriftScore:      math.Round(avgDrift*10) / 10,
				AnswerDirectness: math.Round(avgDirect*10) / 10,
			},
			SampleCount:   len(pairs),
			DriftCount:    driftCount,
			OffTopicCount: offTopicCount,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].RelevanceScore > models[j].RelevanceScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.RelevanceScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostRelevant = models[0].Model
		r.LeastRelevant = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Relevance Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No relevance data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:    %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:          %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most relevant:       %s\n", r.MostRelevant)
	fmt.Fprintf(&b, "  Least relevant:      %s\n\n", r.LeastRelevant)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples)\n",
			m.Model, m.Grade, m.RelevanceScore, m.SampleCount)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "keyword_overlap", m.Detail.KeywordOverlap)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "topic_alignment", m.Detail.TopicAlignment)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "drift_score", m.Detail.DriftScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "answer_directness", m.Detail.AnswerDirectness)
		fmt.Fprintf(&b, "    Drift warnings: %d  Off-topic: %d\n",
			m.DriftCount, m.OffTopicCount)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
