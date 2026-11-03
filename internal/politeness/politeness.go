package politeness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PoliteDetail struct {
	Courtesy     float64 `json:"courtesy"`
	Gratitude    float64 `json:"gratitude"`
	Apology      float64 `json:"apology"`
	Formality    float64 `json:"formality"`
	Patience     float64 `json:"patience"`
	PoliteCount  int     `json:"polite_count"`
	RudeCount    int     `json:"rude_count"`
	NeutralCount int     `json:"neutral_count"`
}

type ModelPolite struct {
	Model          string       `json:"model"`
	PolitenessScore float64    `json:"politeness_score"`
	Grade          string       `json:"grade"`
	Detail         PoliteDetail `json:"detail"`
	SampleCount    int          `json:"sample_count"`
	LabCount       int          `json:"lab_count"`
	Concerns       int          `json:"concerns"`
}

type Report struct {
	Models     []ModelPolite `json:"models"`
	BestModel  string        `json:"best_model"`
	WorstModel string        `json:"worst_model"`
	MeanScore  float64       `json:"mean_score"`
}

var courtesyPhrases = []string{
	"please", "thank you", "you're welcome",
	"if you don't mind", "would you like",
	"may i", "could i", "i'd be happy to",
	"of course", "certainly", "absolutely",
	"with pleasure", "my pleasure",
}

var gratitudePhrases = []string{
	"thank you for", "thanks for", "i appreciate",
	"grateful", "thankful", "much appreciated",
	"that's very kind", "how generous",
}

var apologyPhrases = []string{
	"i apologize", "i'm sorry", "my apologies",
	"pardon me", "excuse me", "forgive me",
	"i regret", "unfortunately",
}

var formalityPhrases = []string{
	"furthermore", "additionally", "moreover",
	"in addition", "consequently", "therefore",
	"nevertheless", "however", "regarding",
	"with respect to", "pertaining to",
}

var patiencePhrases = []string{
	"take your time", "no rush", "whenever you're ready",
	"no problem at all", "happy to wait",
	"let me know when", "at your convenience",
	"feel free to", "don't hesitate",
}

var rudePhrases = []string{
	"obviously", "clearly you", "as i already said",
	"i already told you", "pay attention",
	"that's wrong", "you're wrong", "incorrect",
	"that makes no sense", "ridiculous",
	"that's stupid", "are you serious",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, PoliteDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	court := countPhrases(combined, courtesyPhrases)
	grat := countPhrases(combined, gratitudePhrases)
	apol := countPhrases(combined, apologyPhrases)
	form := countPhrases(combined, formalityPhrases)
	pat := countPhrases(combined, patiencePhrases)
	rude := countPhrases(combined, rudePhrases)

	courtScore := 0.3
	if words > 0 {
		courtScore = math.Min(1.0, float64(court)*2.0/float64(words)*10.0+0.3)
	}
	gratScore := 0.3
	if grat > 0 {
		gratScore = math.Min(1.0, float64(grat)*0.25+0.3)
	}
	apolScore := 0.3
	if apol > 0 {
		apolScore = math.Min(1.0, float64(apol)*0.25+0.3)
	}
	formScore := 0.3
	if form > 0 {
		formScore = math.Min(1.0, float64(form)*0.15+0.3)
	}
	patScore := 0.3
	if pat > 0 {
		patScore = math.Min(1.0, float64(pat)*0.25+0.3)
	}

	politeCount := court + grat + apol + pat
	neutralCount := 0
	if politeCount == 0 && rude == 0 {
		neutralCount = 1
	}

	detail := PoliteDetail{
		Courtesy:     math.Round(courtScore*100) / 100,
		Gratitude:    math.Round(gratScore*100) / 100,
		Apology:      math.Round(apolScore*100) / 100,
		Formality:    math.Round(formScore*100) / 100,
		Patience:     math.Round(patScore*100) / 100,
		PoliteCount:  politeCount,
		RudeCount:    rude,
		NeutralCount: neutralCount,
	}

	score := 45.0
	score += courtScore * 12.0
	score += gratScore * 8.0
	score += apolScore * 8.0
	score += formScore * 8.0
	score += patScore * 10.0
	score -= float64(rude) * 5.0

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
	var models []ModelPolite
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail PoliteDetail
		totalConcerns := 0
		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.Courtesy += detail.Courtesy
			totalDetail.Gratitude += detail.Gratitude
			totalDetail.Apology += detail.Apology
			totalDetail.Formality += detail.Formality
			totalDetail.Patience += detail.Patience
			totalDetail.PoliteCount += detail.PoliteCount
			totalDetail.RudeCount += detail.RudeCount
			totalDetail.NeutralCount += detail.NeutralCount
			if detail.RudeCount > 0 {
				totalConcerns++
			}
		}
		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.Courtesy = math.Round(totalDetail.Courtesy/n*100) / 100
		totalDetail.Gratitude = math.Round(totalDetail.Gratitude/n*100) / 100
		totalDetail.Apology = math.Round(totalDetail.Apology/n*100) / 100
		totalDetail.Formality = math.Round(totalDetail.Formality/n*100) / 100
		totalDetail.Patience = math.Round(totalDetail.Patience/n*100) / 100
		models = append(models, ModelPolite{
			Model:          mn,
			PolitenessScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			Concerns:       totalConcerns,
		})
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].PolitenessScore > models[j].PolitenessScore
	})
	var sumScore float64
	for _, m := range models {
		sumScore += m.PolitenessScore
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
	b.WriteString("Politeness and Courtesy Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")
	if len(r.Models) == 0 {
		b.WriteString("  No politeness data available.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.PolitenessScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "courtesy", m.Detail.Courtesy)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "gratitude", m.Detail.Gratitude)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "apology", m.Detail.Apology)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "formality", m.Detail.Formality)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "patience", m.Detail.Patience)
		fmt.Fprintf(&b, "    %-28s %d\n", "polite_count", m.Detail.PoliteCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "rude_count", m.Detail.RudeCount)
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
