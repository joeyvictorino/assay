package repetition

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type RepetitionDetail struct {
	PhraseRepeat    float64 `json:"phrase_repeat"`
	SentenceRepeat  float64 `json:"sentence_repeat"`
	FillerDensity   float64 `json:"filler_density"`
	CircularLogic   float64 `json:"circular_logic"`
	VerbatimReuse   float64 `json:"verbatim_reuse"`
	RepeatCount     int     `json:"repeat_count"`
	FillerCount     int     `json:"filler_count"`
	UniqueRatio     int     `json:"unique_ratio_pct"`
}

type ModelRepetition struct {
	Model           string           `json:"model"`
	RepetitionScore float64          `json:"repetition_score"`
	Grade           string           `json:"grade"`
	Detail          RepetitionDetail `json:"detail"`
	SampleCount     int              `json:"sample_count"`
	LabCount        int              `json:"lab_count"`
	Concerns        int              `json:"concerns"`
}

type Report struct {
	Models     []ModelRepetition `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var fillerPhrases = []string{
	"as i mentioned", "as stated above", "as previously noted",
	"as we discussed", "to reiterate", "once again",
	"in other words", "that is to say", "to put it another way",
	"basically", "essentially", "fundamentally",
	"it is important to note", "it should be noted",
}

var circularPhrases = []string{
	"this is because", "the reason is",
	"which means that", "in conclusion",
	"therefore we can see", "as we can see",
	"this demonstrates that", "this shows that",
	"going back to", "returning to",
	"as mentioned earlier", "as noted before",
}

var redundantPhrases = []string{
	"each and every", "first and foremost", "various different",
	"past history", "future plans", "end result",
	"completely eliminate", "absolutely essential",
	"basic fundamentals", "close proximity",
	"final conclusion", "free gift",
}

var paddingPhrases = []string{
	"it goes without saying", "needless to say",
	"at the end of the day", "when all is said and done",
	"the fact of the matter", "in point of fact",
	"for all intents and purposes", "by and large",
	"all things considered", "in the final analysis",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func countRepeatedSentences(text string) (int, int) {
	sentences := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})
	seen := map[string]int{}
	for _, s := range sentences {
		trimmed := strings.TrimSpace(strings.ToLower(s))
		if len(trimmed) > 10 {
			seen[trimmed]++
		}
	}
	total := len(seen)
	repeated := 0
	for _, c := range seen {
		if c > 1 {
			repeated++
		}
	}
	return repeated, total
}

func uniqueWordRatio(text string) float64 {
	words := strings.Fields(strings.ToLower(text))
	if len(words) == 0 {
		return 1.0
	}
	unique := map[string]bool{}
	for _, w := range words {
		unique[w] = true
	}
	return float64(len(unique)) / float64(len(words))
}

func scoreResponse(response, evidence string) (float64, RepetitionDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	fillers := countPhrases(combined, fillerPhrases)
	circular := countPhrases(combined, circularPhrases)
	redundant := countPhrases(combined, redundantPhrases)
	padding := countPhrases(combined, paddingPhrases)

	repeatedSent, totalSent := countRepeatedSentences(combined)
	uRatio := uniqueWordRatio(combined)

	phraseRepeatScore := 0.9
	if words > 0 {
		density := float64(redundant+padding) / float64(words) * 100
		phraseRepeatScore = math.Max(0, 0.9-density*0.15)
	}

	sentRepeatScore := 0.9
	if totalSent > 0 {
		sentRepeatScore = math.Max(0, 0.9-float64(repeatedSent)*0.2)
	}

	fillerScore := 0.9
	if words > 0 {
		fillerScore = math.Max(0, 0.9-float64(fillers)/float64(words)*50)
	}

	circularScore := 0.9
	if circular > 0 {
		circularScore = math.Max(0, 0.9-float64(circular)*0.15)
	}

	verbatimScore := math.Min(1.0, uRatio+0.1)

	totalRepeat := fillers + redundant + padding
	uniquePct := int(math.Round(uRatio * 100))

	detail := RepetitionDetail{
		PhraseRepeat:   math.Round(phraseRepeatScore*100) / 100,
		SentenceRepeat: math.Round(sentRepeatScore*100) / 100,
		FillerDensity:  math.Round(fillerScore*100) / 100,
		CircularLogic:  math.Round(circularScore*100) / 100,
		VerbatimReuse:  math.Round(verbatimScore*100) / 100,
		RepeatCount:    totalRepeat,
		FillerCount:    fillers,
		UniqueRatio:    uniquePct,
	}

	score := 40.0
	score += phraseRepeatScore * 12.0
	score += sentRepeatScore * 12.0
	score += fillerScore * 10.0
	score += circularScore * 10.0
	score += verbatimScore * 10.0

	score -= float64(totalRepeat) * 2.0

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

	var models []ModelRepetition
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail RepetitionDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.PhraseRepeat += detail.PhraseRepeat
			totalDetail.SentenceRepeat += detail.SentenceRepeat
			totalDetail.FillerDensity += detail.FillerDensity
			totalDetail.CircularLogic += detail.CircularLogic
			totalDetail.VerbatimReuse += detail.VerbatimReuse
			totalDetail.RepeatCount += detail.RepeatCount
			totalDetail.FillerCount += detail.FillerCount
			totalDetail.UniqueRatio += detail.UniqueRatio

			if detail.RepeatCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.PhraseRepeat = math.Round(totalDetail.PhraseRepeat/n*100) / 100
		totalDetail.SentenceRepeat = math.Round(totalDetail.SentenceRepeat/n*100) / 100
		totalDetail.FillerDensity = math.Round(totalDetail.FillerDensity/n*100) / 100
		totalDetail.CircularLogic = math.Round(totalDetail.CircularLogic/n*100) / 100
		totalDetail.VerbatimReuse = math.Round(totalDetail.VerbatimReuse/n*100) / 100
		totalDetail.UniqueRatio = int(math.Round(float64(totalDetail.UniqueRatio) / n))

		models = append(models, ModelRepetition{
			Model:           mn,
			RepetitionScore: math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			Concerns:        totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].RepetitionScore > models[j].RepetitionScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.RepetitionScore
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
	b.WriteString("Repetition and Redundancy Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No repetition data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.RepetitionScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "phrase_repeat", m.Detail.PhraseRepeat)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "sentence_repeat", m.Detail.SentenceRepeat)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "filler_density", m.Detail.FillerDensity)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "circular_logic", m.Detail.CircularLogic)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "verbatim_reuse", m.Detail.VerbatimReuse)
		fmt.Fprintf(&b, "    %-28s %d\n", "repeat_count", m.Detail.RepeatCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "filler_count", m.Detail.FillerCount)
		fmt.Fprintf(&b, "    %-28s %d%%\n", "unique_ratio", m.Detail.UniqueRatio)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
