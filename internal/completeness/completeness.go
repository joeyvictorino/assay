package completeness

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type CompletenessDetail struct {
	WordCount       int     `json:"word_count"`
	QuestionsCaught int     `json:"questions_caught"`
	QuestionsTotal  int     `json:"questions_total"`
	TopicsCovered   int     `json:"topics_covered"`
	ListItems       int     `json:"list_items"`
	SectionCount    int     `json:"section_count"`
	CoverageRatio   float64 `json:"coverage_ratio"`
}

type ModelCompleteness struct {
	Model       string             `json:"model"`
	Score       float64            `json:"score"`
	Grade       string             `json:"grade"`
	Detail      CompletenessDetail `json:"detail"`
	SampleCount int                `json:"sample_count"`
	LabCount    int                `json:"lab_count"`
}

type Report struct {
	Models     []ModelCompleteness `json:"models"`
	BestModel  string              `json:"best_model"`
	WorstModel string              `json:"worst_model"`
	MeanScore  float64             `json:"mean_score"`
}

var questionPattern = regexp.MustCompile(`(?i)\b(what|how|why|when|where|which|who|can|does|is|are|should|would|could)\b`)
var listPattern = regexp.MustCompile(`(?m)^[\s]*(?:[-*]\s|\d+[.)]\s)`)
var sectionPattern = regexp.MustCompile(`(?m)^#+\s|^[A-Z][^.!?]*:\s*$`)

func extractQuestionWords(prompt string) []string {
	matches := questionPattern.FindAllString(strings.ToLower(prompt), -1)
	seen := map[string]bool{}
	var unique []string
	for _, m := range matches {
		m = strings.ToLower(m)
		if !seen[m] {
			seen[m] = true
			unique = append(unique, m)
		}
	}
	return unique
}

func countAnsweredQuestions(prompt, response string) (int, int) {
	questions := extractQuestionWords(prompt)
	total := len(questions)
	if total == 0 {
		return 0, 0
	}
	answered := 0
	lower := strings.ToLower(response)
	for _, q := range questions {
		if strings.Contains(lower, q) {
			answered++
		}
	}
	return answered, total
}

func countListItems(text string) int {
	return len(listPattern.FindAllString(text, -1))
}

func countSections(text string) int {
	return len(sectionPattern.FindAllString(text, -1))
}

func extractTopics(prompt string) []string {
	words := strings.Fields(strings.ToLower(prompt))
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "can": true, "shall": true, "to": true,
		"of": true, "in": true, "for": true, "on": true, "with": true,
		"at": true, "by": true, "from": true, "and": true, "or": true,
		"but": true, "not": true, "this": true, "that": true, "it": true,
		"what": true, "how": true, "why": true, "when": true, "where": true,
		"which": true, "who": true, "i": true, "you": true, "we": true,
		"they": true, "me": true, "my": true, "your": true, "if": true,
		"about": true, "up": true, "out": true, "so": true, "no": true,
	}
	var topics []string
	seen := map[string]bool{}
	for _, w := range words {
		cleaned := strings.TrimFunc(w, func(r rune) bool {
			return r < 'a' || r > 'z'
		})
		if len(cleaned) > 2 && !stopWords[cleaned] && !seen[cleaned] {
			seen[cleaned] = true
			topics = append(topics, cleaned)
		}
	}
	return topics
}

func countTopicsCovered(prompt, response string) int {
	topics := extractTopics(prompt)
	lower := strings.ToLower(response)
	covered := 0
	for _, t := range topics {
		if strings.Contains(lower, t) {
			covered++
		}
	}
	return covered
}

func AnalyzeText(prompt, response string) (float64, CompletenessDetail) {
	words := strings.Fields(response)
	wordCount := len(words)

	if wordCount == 0 {
		return 0.0, CompletenessDetail{}
	}

	answered, total := countAnsweredQuestions(prompt, response)
	topicsCovered := countTopicsCovered(prompt, response)
	listItems := countListItems(response)
	sections := countSections(response)

	score := 50.0

	if total > 0 {
		ratio := float64(answered) / float64(total)
		score += ratio * 25.0
	}

	topicCount := len(extractTopics(prompt))
	if topicCount > 0 {
		topicRatio := float64(topicsCovered) / float64(topicCount)
		score += topicRatio * 15.0
	}

	if listItems > 0 {
		score += math.Min(float64(listItems)*2.0, 10.0)
	}

	if sections > 0 {
		score += math.Min(float64(sections)*2.0, 8.0)
	}

	lengthBonus := math.Min(float64(wordCount)/10.0, 5.0)
	score += lengthBonus

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	coverageRatio := 0.0
	if total > 0 {
		coverageRatio = float64(answered) / float64(total)
	}

	detail := CompletenessDetail{
		WordCount:       wordCount,
		QuestionsCaught: answered,
		QuestionsTotal:  total,
		TopicsCovered:   topicsCovered,
		ListItems:       listItems,
		SectionCount:    sections,
		CoverageRatio:   math.Round(coverageRatio*100) / 100,
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

	var models []ModelCompleteness
	for mn, entries := range modelEntries {
		var totalScore float64
		var aggDetail CompletenessDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := AnalyzeText(e.Lab, e.Summary)
			totalScore += score
			aggDetail.WordCount += detail.WordCount
			aggDetail.QuestionsCaught += detail.QuestionsCaught
			aggDetail.QuestionsTotal += detail.QuestionsTotal
			aggDetail.TopicsCovered += detail.TopicsCovered
			aggDetail.ListItems += detail.ListItems
			aggDetail.SectionCount += detail.SectionCount
			aggDetail.CoverageRatio += detail.CoverageRatio
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		aggDetail.CoverageRatio = math.Round(aggDetail.CoverageRatio/n*100) / 100

		models = append(models, ModelCompleteness{
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
	b.WriteString("Response Completeness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No completeness data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s\n", m.Score, m.Grade)
		fmt.Fprintf(&b, "    Questions: %d/%d  Topics: %d  Lists: %d  Sections: %d\n",
			m.Detail.QuestionsCaught, m.Detail.QuestionsTotal,
			m.Detail.TopicsCovered, m.Detail.ListItems, m.Detail.SectionCount)
		fmt.Fprintf(&b, "    Samples: %d  Labs: %d\n\n", m.SampleCount, m.LabCount)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
