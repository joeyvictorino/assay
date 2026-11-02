package verbosityscore

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var paddingPhrases = []string{
	"it is important to note that",
	"it should be noted that",
	"it is worth mentioning",
	"as mentioned earlier",
	"as previously stated",
	"in other words",
	"to put it simply",
	"to be more specific",
	"having said that",
	"that being said",
	"with that being said",
	"i would like to point out",
	"please note that",
	"keep in mind that",
	"it goes without saying",
	"needless to say",
	"as a matter of fact",
	"for what it is worth",
	"at the end of the day",
	"all things considered",
}

var disclaimerPhrases = []string{
	"i cannot",
	"i'm not able to",
	"i am not able to",
	"as an ai",
	"as a language model",
	"i don't have access",
	"i do not have access",
	"please consult",
	"please note",
	"disclaimer",
	"caveat",
	"important to remember",
	"i should mention",
	"i must emphasize",
}

type VerbosityBreakdown struct {
	LengthScore    float64 `json:"length_score"`
	PaddingScore   float64 `json:"padding_score"`
	DisclaimerScore float64 `json:"disclaimer_score"`
	RepetitionScore float64 `json:"repetition_score"`
}

type ModelVerbosity struct {
	Model          string             `json:"model"`
	VerbosityScore float64            `json:"verbosity_score"`
	Grade          string             `json:"grade"`
	Breakdown      VerbosityBreakdown `json:"breakdown"`
	SampleCount    int                `json:"sample_count"`
	AvgWordCount   float64            `json:"avg_word_count"`
	PaddingCount   int                `json:"padding_count"`
	DisclaimerCount int               `json:"disclaimer_count"`
}

type Report struct {
	Models        []ModelVerbosity `json:"models"`
	BestModel     string           `json:"best_model"`
	WorstModel    string           `json:"worst_model"`
	MeanScore     float64          `json:"mean_score"`
	MeanWordCount float64          `json:"mean_word_count"`
}

func countPaddingPhrases(text string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range paddingPhrases {
		idx := 0
		for {
			pos := strings.Index(lower[idx:], p)
			if pos == -1 {
				break
			}
			count++
			idx += pos + len(p)
			if idx >= len(lower) {
				break
			}
		}
	}
	return count
}

func countDisclaimerPhrases(text string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, d := range disclaimerPhrases {
		idx := 0
		for {
			pos := strings.Index(lower[idx:], d)
			if pos == -1 {
				break
			}
			count++
			idx += pos + len(d)
			if idx >= len(lower) {
				break
			}
		}
	}
	return count
}

func lengthScore(texts []string) float64 {
	if len(texts) == 0 {
		return 50
	}
	var totalWords int
	for _, t := range texts {
		totalWords += len(strings.Fields(t))
	}
	avg := float64(totalWords) / float64(len(texts))
	if avg <= 50 {
		return 90
	}
	if avg <= 100 {
		return 80
	}
	if avg <= 200 {
		return 70
	}
	if avg <= 400 {
		return 50
	}
	penalty := math.Min(50, (avg-400)/20)
	return math.Max(0, 50-penalty)
}

func paddingScore(texts []string) float64 {
	if len(texts) == 0 {
		return 100
	}
	totalPadding := 0
	totalWords := 0
	for _, t := range texts {
		totalPadding += countPaddingPhrases(t)
		totalWords += len(strings.Fields(t))
	}
	if totalWords == 0 {
		return 100
	}
	ratio := float64(totalPadding) / float64(len(texts))
	score := 100 - math.Min(100, ratio*25)
	return math.Round(score*10) / 10
}

func disclaimerScore(texts []string) float64 {
	if len(texts) == 0 {
		return 100
	}
	totalDisclaimer := 0
	for _, t := range texts {
		totalDisclaimer += countDisclaimerPhrases(t)
	}
	ratio := float64(totalDisclaimer) / float64(len(texts))
	score := 100 - math.Min(100, ratio*20)
	return math.Round(score*10) / 10
}

func repetitionScore(texts []string) float64 {
	if len(texts) == 0 {
		return 100
	}
	var scores []float64
	for _, t := range texts {
		words := strings.Fields(strings.ToLower(t))
		if len(words) < 5 {
			scores = append(scores, 100)
			continue
		}
		trigrams := map[string]int{}
		for i := 0; i+2 < len(words); i++ {
			tri := words[i] + " " + words[i+1] + " " + words[i+2]
			trigrams[tri]++
		}
		repeated := 0
		total := len(words) - 2
		for _, c := range trigrams {
			if c > 1 {
				repeated += c - 1
			}
		}
		if total <= 0 {
			scores = append(scores, 100)
			continue
		}
		ratio := float64(repeated) / float64(total)
		s := 100 - math.Min(100, ratio*300)
		if s < 0 {
			s = 0
		}
		scores = append(scores, s)
	}
	var sum float64
	for _, s := range scores {
		sum += s
	}
	return math.Round(sum/float64(len(scores))*10) / 10
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

	modelTexts := map[string][]string{}
	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		modelTexts[mn] = append(modelTexts[mn], f.Summary)
	}

	var models []ModelVerbosity
	for mn, texts := range modelTexts {
		ls := lengthScore(texts)
		ps := paddingScore(texts)
		ds := disclaimerScore(texts)
		rs := repetitionScore(texts)

		overall := (ls + ps + ds + rs) / 4

		var totalWords int
		totalPadding := 0
		totalDisclaimer := 0
		for _, t := range texts {
			totalWords += len(strings.Fields(t))
			totalPadding += countPaddingPhrases(t)
			totalDisclaimer += countDisclaimerPhrases(t)
		}

		models = append(models, ModelVerbosity{
			Model:          mn,
			VerbosityScore: math.Round(overall*10) / 10,
			Grade:          scoreToGrade(overall),
			Breakdown: VerbosityBreakdown{
				LengthScore:    ls,
				PaddingScore:   ps,
				DisclaimerScore: ds,
				RepetitionScore: rs,
			},
			SampleCount:    len(texts),
			AvgWordCount:   math.Round(float64(totalWords)/float64(len(texts))*10) / 10,
			PaddingCount:   totalPadding,
			DisclaimerCount: totalDisclaimer,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].VerbosityScore > models[j].VerbosityScore
	})

	var sumScore, sumWords float64
	for _, m := range models {
		sumScore += m.VerbosityScore
		sumWords += m.AvgWordCount
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
		r.MeanWordCount = math.Round(sumWords/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Verbosity Score Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No verbosity data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:    %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:          %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Mean word count:     %.1f\n", r.MeanWordCount)
	fmt.Fprintf(&b, "  Least verbose:       %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Most verbose:        %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples)\n",
			m.Model, m.Grade, m.VerbosityScore, m.SampleCount)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "length_score", m.Breakdown.LengthScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "padding_score", m.Breakdown.PaddingScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "disclaimer_score", m.Breakdown.DisclaimerScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "repetition_score", m.Breakdown.RepetitionScore)
		fmt.Fprintf(&b, "    Avg words: %.1f  Padding: %d  Disclaimers: %d\n",
			m.AvgWordCount, m.PaddingCount, m.DisclaimerCount)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
