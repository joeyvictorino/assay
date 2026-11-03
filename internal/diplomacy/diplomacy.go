package diplomacy

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type DiplomacyDetail struct {
	Tact             float64 `json:"tact"`
	Neutrality       float64 `json:"neutrality"`
	Balance          float64 `json:"balance"`
	Constructiveness float64 `json:"constructiveness"`
	Acknowledgment   float64 `json:"acknowledgment"`
	DiplomaticCount  int     `json:"diplomatic_count"`
	AggressiveCount  int     `json:"aggressive_count"`
	NeutralCount     int     `json:"neutral_count"`
}

type ModelDiplomacy struct {
	Model          string          `json:"model"`
	DiplomacyScore float64         `json:"diplomacy_score"`
	Grade          string          `json:"grade"`
	Detail         DiplomacyDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	LabCount       int             `json:"lab_count"`
	Concerns       int             `json:"concerns"`
}

type Report struct {
	Models     []ModelDiplomacy `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var tactPhrases = []string{
	"i understand your perspective", "that's a fair point",
	"i see where you're coming from", "you raise a valid concern",
	"i can appreciate that", "that's worth considering",
	"let me consider that", "i respect your view",
	"that's an interesting angle", "you make a good point",
	"i hear you", "that's understandable",
}

var neutralityPhrases = []string{
	"on the other hand", "from another perspective",
	"some might argue", "it depends on",
	"there are different views", "both sides have merit",
	"it's nuanced", "the situation is complex",
	"in fairness", "to be fair",
	"objectively speaking", "without bias",
}

var balancePhrases = []string{
	"while that's true", "at the same time",
	"however it's also", "that said",
	"on balance", "taking everything into account",
	"considering all factors", "weighing the options",
	"pros and cons", "advantages and disadvantages",
}

var constructivePhrases = []string{
	"one suggestion would be", "you might consider",
	"an alternative approach", "perhaps we could",
	"what if we", "it might help to",
	"a possible solution", "we could try",
	"building on that", "to improve this",
	"moving forward", "as a next step",
}

var acknowledgmentPhrases = []string{
	"i acknowledge", "you're right that",
	"that's correct", "good observation",
	"well said", "exactly right",
	"i agree with", "you're absolutely right",
	"that's a great point", "well noted",
}

var aggressivePhrases = []string{
	"you're completely wrong", "that's nonsense",
	"don't be ridiculous", "that's absurd",
	"you clearly don't understand", "how could you think",
	"that's the worst", "absolutely not",
	"you have no idea", "stop wasting time",
	"this is unacceptable", "you failed to",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, DiplomacyDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	tact := countPhrases(combined, tactPhrases)
	neut := countPhrases(combined, neutralityPhrases)
	bal := countPhrases(combined, balancePhrases)
	cons := countPhrases(combined, constructivePhrases)
	ack := countPhrases(combined, acknowledgmentPhrases)
	aggr := countPhrases(combined, aggressivePhrases)

	tactScore := 0.3
	if words > 0 {
		tactScore = math.Min(1.0, float64(tact)*2.0/float64(words)*10.0+0.3)
	}
	neutScore := 0.3
	if neut > 0 {
		neutScore = math.Min(1.0, float64(neut)*0.25+0.3)
	}
	balScore := 0.3
	if bal > 0 {
		balScore = math.Min(1.0, float64(bal)*0.25+0.3)
	}
	consScore := 0.3
	if cons > 0 {
		consScore = math.Min(1.0, float64(cons)*0.2+0.3)
	}
	ackScore := 0.3
	if ack > 0 {
		ackScore = math.Min(1.0, float64(ack)*0.25+0.3)
	}

	diplomaticCount := tact + neut + bal + cons + ack
	neutralCount := 0
	if diplomaticCount == 0 && aggr == 0 {
		neutralCount = 1
	}

	detail := DiplomacyDetail{
		Tact:             math.Round(tactScore*100) / 100,
		Neutrality:       math.Round(neutScore*100) / 100,
		Balance:          math.Round(balScore*100) / 100,
		Constructiveness: math.Round(consScore*100) / 100,
		Acknowledgment:   math.Round(ackScore*100) / 100,
		DiplomaticCount:  diplomaticCount,
		AggressiveCount:  aggr,
		NeutralCount:     neutralCount,
	}

	score := 45.0
	score += tactScore * 12.0
	score += neutScore * 10.0
	score += balScore * 8.0
	score += consScore * 10.0
	score += ackScore * 8.0
	score -= float64(aggr) * 6.0

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
	var models []ModelDiplomacy
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail DiplomacyDetail
		totalConcerns := 0
		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.Tact += detail.Tact
			totalDetail.Neutrality += detail.Neutrality
			totalDetail.Balance += detail.Balance
			totalDetail.Constructiveness += detail.Constructiveness
			totalDetail.Acknowledgment += detail.Acknowledgment
			totalDetail.DiplomaticCount += detail.DiplomaticCount
			totalDetail.AggressiveCount += detail.AggressiveCount
			totalDetail.NeutralCount += detail.NeutralCount
			if detail.AggressiveCount > 0 {
				totalConcerns++
			}
		}
		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.Tact = math.Round(totalDetail.Tact/n*100) / 100
		totalDetail.Neutrality = math.Round(totalDetail.Neutrality/n*100) / 100
		totalDetail.Balance = math.Round(totalDetail.Balance/n*100) / 100
		totalDetail.Constructiveness = math.Round(totalDetail.Constructiveness/n*100) / 100
		totalDetail.Acknowledgment = math.Round(totalDetail.Acknowledgment/n*100) / 100
		models = append(models, ModelDiplomacy{
			Model:          mn,
			DiplomacyScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			Concerns:       totalConcerns,
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].DiplomacyScore > models[j].DiplomacyScore
	})
	var sumScore float64
	for _, m := range models {
		sumScore += m.DiplomacyScore
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
	b.WriteString("Diplomatic Communication Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")
	if len(r.Models) == 0 {
		b.WriteString("  No diplomacy data available.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.DiplomacyScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "tact", m.Detail.Tact)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "neutrality", m.Detail.Neutrality)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "balance", m.Detail.Balance)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "constructiveness", m.Detail.Constructiveness)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "acknowledgment", m.Detail.Acknowledgment)
		fmt.Fprintf(&b, "    %-28s %d\n", "diplomatic_count", m.Detail.DiplomaticCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "aggressive_count", m.Detail.AggressiveCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "neutral_count", m.Detail.NeutralCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
