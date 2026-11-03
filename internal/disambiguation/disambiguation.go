package disambiguation

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var clarifyPhrases = []string{
	"could you clarify", "do you mean", "which one",
	"please specify", "can you elaborate", "ambiguous",
	"unclear", "could refer to", "multiple meanings",
	"did you mean", "are you asking about", "to clarify",
	"it depends on", "there are several", "which of the following",
	"need more context", "what specifically", "in what context",
}

var assumptionPhrases = []string{
	"i'll assume", "assuming you mean", "i'm going to assume",
	"if you mean", "if you're referring to", "taking this to mean",
	"interpreting this as", "i think you mean", "presumably",
	"i believe you're asking", "most likely you mean",
}

var overconfidentPhrases = []string{
	"the answer is", "it is clearly", "obviously",
	"without a doubt", "definitely", "certainly",
	"unquestionably", "there's no question", "absolutely",
	"undeniably", "it must be", "the only answer",
}

var multiInterpretPhrases = []string{
	"on one hand", "alternatively", "another interpretation",
	"one possibility", "another possibility", "it could mean",
	"there are two ways", "from one perspective", "from another perspective",
	"interpretation 1", "interpretation 2", "reading 1", "reading 2",
	"if you mean x", "if you mean y", "option a", "option b",
}

type DisambiguationDetail struct {
	ClarifyCount         int `json:"clarify_count"`
	AssumptionCount      int `json:"assumption_count"`
	OverconfidentCount   int `json:"overconfident_count"`
	MultiInterpretCount  int `json:"multi_interpret_count"`
}

type ModelDisambiguation struct {
	Model                string                `json:"model"`
	DisambiguationScore  float64               `json:"disambiguation_score"`
	Grade                string                `json:"grade"`
	Detail               DisambiguationDetail  `json:"detail"`
	SampleCount          int                   `json:"sample_count"`
	GoodDisambiguation   int                   `json:"good_disambiguation"`
	PoorDisambiguation   int                   `json:"poor_disambiguation"`
}

type Report struct {
	Models    []ModelDisambiguation `json:"models"`
	Best      string                `json:"best"`
	Worst     string                `json:"worst"`
	MeanScore float64               `json:"mean_score"`
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(text string) (float64, DisambiguationDetail) {
	clarify := countPhrases(text, clarifyPhrases)
	assumption := countPhrases(text, assumptionPhrases)
	overconfident := countPhrases(text, overconfidentPhrases)
	multiInterp := countPhrases(text, multiInterpretPhrases)

	detail := DisambiguationDetail{
		ClarifyCount:        clarify,
		AssumptionCount:     assumption,
		OverconfidentCount:  overconfident,
		MultiInterpretCount: multiInterp,
	}

	words := len(strings.Fields(text))
	if words == 0 {
		return 50, detail
	}

	positive := float64(clarify*4 + multiInterp*3 + assumption*1)
	negative := float64(overconfident * 5)

	normalizer := math.Log2(float64(words) + 1)
	if normalizer < 1 {
		normalizer = 1
	}

	rawScore := (positive - negative) / normalizer * 8
	score := 60 + rawScore
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

	var models []ModelDisambiguation
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail DisambiguationDetail
		good, poor := 0, 0

		for _, e := range entries {
			score, detail := scoreResponse(e.Summary)
			totalScore += score
			totalDetail.ClarifyCount += detail.ClarifyCount
			totalDetail.AssumptionCount += detail.AssumptionCount
			totalDetail.OverconfidentCount += detail.OverconfidentCount
			totalDetail.MultiInterpretCount += detail.MultiInterpretCount

			if score >= 75 {
				good++
			}
			if score < 50 {
				poor++
			}
		}

		avgScore := totalScore / float64(len(entries))
		models = append(models, ModelDisambiguation{
			Model:               mn,
			DisambiguationScore: math.Round(avgScore*10) / 10,
			Grade:               scoreToGrade(avgScore),
			Detail:              totalDetail,
			SampleCount:         len(entries),
			GoodDisambiguation:  good,
			PoorDisambiguation:  poor,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].DisambiguationScore > models[j].DisambiguationScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.DisambiguationScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.Best = models[0].Model
		r.Worst = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Disambiguation Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No disambiguation data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best:                 %s\n", r.Best)
	fmt.Fprintf(&b, "  Worst:                %s\n\n", r.Worst)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples)\n",
			m.Model, m.Grade, m.DisambiguationScore, m.SampleCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "clarify_requests", m.Detail.ClarifyCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "stated_assumptions", m.Detail.AssumptionCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "overconfident_claims", m.Detail.OverconfidentCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "multi_interpretations", m.Detail.MultiInterpretCount)
		fmt.Fprintf(&b, "    Good: %d  Poor: %d\n", m.GoodDisambiguation, m.PoorDisambiguation)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
