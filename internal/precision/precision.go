package precision

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PrecisionDetail struct {
	DirectAnswerCount int     `json:"direct_answer_count"`
	SpecificRefCount  int     `json:"specific_ref_count"`
	TangentCount      int     `json:"tangent_count"`
	FillerCount       int     `json:"filler_count"`
	OverExplainCount  int     `json:"over_explain_count"`
	PrecisionRatio    float64 `json:"precision_ratio"`
	WordCount         int     `json:"word_count"`
}

type ModelPrecision struct {
	Model          string          `json:"model"`
	PrecisionScore float64         `json:"precision_score"`
	Grade          string          `json:"grade"`
	Detail         PrecisionDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	LabCount       int             `json:"lab_count"`
}

type Report struct {
	Models     []ModelPrecision `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var directAnswerPhrases = []string{
	"the answer is", "specifically", "in short",
	"to directly answer", "the result is", "precisely",
	"exactly", "the key point is", "in summary",
	"to be specific", "the solution is", "concretely",
	"the direct cause", "narrowly", "succinctly",
}

var specificRefPhrases = []string{
	"as shown in", "according to", "based on the data",
	"the evidence shows", "referencing", "as documented",
	"per the specification", "as defined in", "citing",
	"from the source", "as measured by", "validated by",
	"as demonstrated", "confirmed by", "cross-referenced",
}

var tangentPhrases = []string{
	"by the way", "on a related note", "incidentally",
	"speaking of which", "as an aside", "while we're at it",
	"it's worth mentioning", "tangentially", "on another note",
	"as a side note", "unrelated but", "off topic",
}

var fillerPhrases = []string{
	"as you may know", "it goes without saying",
	"needless to say", "obviously", "of course",
	"as everyone knows", "naturally", "clearly",
	"it is well known", "as always", "as usual",
	"without a doubt", "undeniably", "inevitably",
}

var overExplainPhrases = []string{
	"to elaborate further", "in other words",
	"let me explain in more detail", "to put it another way",
	"what i mean is", "to clarify further",
	"breaking this down further", "expanding on this",
	"to reiterate", "once again", "as mentioned before",
	"to repeat", "restating",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, PrecisionDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	direct := countPhrases(combined, directAnswerPhrases)
	specific := countPhrases(combined, specificRefPhrases)
	tangent := countPhrases(combined, tangentPhrases)
	filler := countPhrases(combined, fillerPhrases)
	overExplain := countPhrases(combined, overExplainPhrases)

	precisionSignals := float64(direct + specific)
	noiseSignals := float64(tangent + filler + overExplain)

	ratio := 0.5
	if precisionSignals+noiseSignals > 0 {
		ratio = precisionSignals / (precisionSignals + noiseSignals)
	}

	detail := PrecisionDetail{
		DirectAnswerCount: direct,
		SpecificRefCount:  specific,
		TangentCount:      tangent,
		FillerCount:       filler,
		OverExplainCount:  overExplain,
		PrecisionRatio:    math.Round(ratio*100) / 100,
		WordCount:         words,
	}

	score := 40.0
	score += ratio * 40.0
	score += math.Min(float64(direct)*3.0, 15.0)
	score += math.Min(float64(specific)*2.0, 10.0)
	score -= float64(tangent) * 4.0
	score -= float64(filler) * 3.0
	score -= float64(overExplain) * 2.0

	if words > 500 && precisionSignals < 2 {
		score -= 10.0
	}

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

	var models []ModelPrecision
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail PrecisionDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.DirectAnswerCount += detail.DirectAnswerCount
			totalDetail.SpecificRefCount += detail.SpecificRefCount
			totalDetail.TangentCount += detail.TangentCount
			totalDetail.FillerCount += detail.FillerCount
			totalDetail.OverExplainCount += detail.OverExplainCount
			totalDetail.WordCount += detail.WordCount
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.PrecisionRatio = math.Round(totalDetail.PrecisionRatio/n*100) / 100

		models = append(models, ModelPrecision{
			Model:          mn,
			PrecisionScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].PrecisionScore > models[j].PrecisionScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.PrecisionScore
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
	b.WriteString("Response Precision Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No precision data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.PrecisionScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "direct_answer_count", m.Detail.DirectAnswerCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "specific_ref_count", m.Detail.SpecificRefCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "tangent_count", m.Detail.TangentCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "filler_count", m.Detail.FillerCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "over_explain_count", m.Detail.OverExplainCount)
		b.WriteString("\n")
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
