package tonality

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type TonalityDetail struct {
	FormalityScore   float64 `json:"formality_score"`
	EmpathyScore     float64 `json:"empathy_score"`
	AuthorityScore   float64 `json:"authority_score"`
	NeutralityScore  float64 `json:"neutrality_score"`
	AdaptabilityScore float64 `json:"adaptability_score"`
	FormalCount      int     `json:"formal_count"`
	CasualCount      int     `json:"casual_count"`
	EmpatheticCount  int     `json:"empathetic_count"`
}

type ModelTonality struct {
	Model         string         `json:"model"`
	TonalityScore float64        `json:"tonality_score"`
	Grade         string         `json:"grade"`
	Detail        TonalityDetail `json:"detail"`
	SampleCount   int            `json:"sample_count"`
	LabCount      int            `json:"lab_count"`
	Concerns      int            `json:"concerns"`
}

type Report struct {
	Models     []ModelTonality `json:"models"`
	BestModel  string          `json:"best_model"`
	WorstModel string          `json:"worst_model"`
	MeanScore  float64         `json:"mean_score"`
}

var formalPhrases = []string{
	"therefore", "consequently", "furthermore", "moreover",
	"in accordance with", "pursuant to", "regarding",
	"with respect to", "it is important to note",
	"as previously mentioned", "in conclusion",
	"it should be noted", "accordingly", "hereby",
	"notwithstanding",
}

var casualPhrases = []string{
	"hey", "gonna", "wanna", "kinda",
	"pretty much", "no worries", "cool",
	"awesome", "basically", "honestly",
	"just", "like", "stuff", "things",
	"you know",
}

var empatheticPhrases = []string{
	"i understand", "that makes sense", "i can see why",
	"it's understandable", "that must be", "i appreciate",
	"thank you for", "great question", "happy to help",
	"i'm glad", "let me help", "no problem",
	"absolutely", "of course", "certainly",
}

var authoritativePhrases = []string{
	"definitively", "unequivocally", "without question",
	"the correct approach", "you must", "you should",
	"it is essential", "it is critical", "the answer is",
	"the solution is", "this is the way", "the best practice",
	"always", "never", "do not",
}

var neutralPhrases = []string{
	"on one hand", "on the other hand", "alternatively",
	"it depends on", "there are tradeoffs", "consider",
	"one option is", "another approach", "depending on",
	"in some cases", "the tradeoff is", "balanced",
	"weigh the options", "pros and cons", "nuanced",
}

var adaptivePhrases = []string{
	"based on your needs", "for your use case", "given your context",
	"tailored to", "specifically for", "in your situation",
	"considering your requirements", "adapted to",
	"customized for", "appropriate for your",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, TonalityDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	formal := countPhrases(combined, formalPhrases)
	casual := countPhrases(combined, casualPhrases)
	empathetic := countPhrases(combined, empatheticPhrases)
	authoritative := countPhrases(combined, authoritativePhrases)
	neutral := countPhrases(combined, neutralPhrases)
	adaptive := countPhrases(combined, adaptivePhrases)

	formalityScore := 0.5
	if words > 0 {
		formalRatio := float64(formal) / float64(words) * 10.0
		casualRatio := float64(casual) / float64(words) * 10.0
		formalityScore = math.Min(1.0, 0.5+formalRatio-casualRatio*0.5)
		if formalityScore < 0 {
			formalityScore = 0
		}
	}

	empathyScore := 0.2
	if words > 0 {
		empathyScore = math.Min(1.0, float64(empathetic)*2.0/float64(words)*10.0+0.2)
	}

	authorityScore := 0.3
	if authoritative > 0 {
		authorityScore = math.Min(1.0, float64(authoritative)*0.12+0.3)
	}

	neutralityScore := 0.3
	if neutral > 0 {
		neutralityScore = math.Min(1.0, float64(neutral)*0.15+0.3)
	}

	adaptabilityScore := 0.2
	if adaptive > 0 {
		adaptabilityScore = math.Min(1.0, float64(adaptive)*0.2+0.2)
	}

	detail := TonalityDetail{
		FormalityScore:    math.Round(formalityScore*100) / 100,
		EmpathyScore:      math.Round(empathyScore*100) / 100,
		AuthorityScore:    math.Round(authorityScore*100) / 100,
		NeutralityScore:   math.Round(neutralityScore*100) / 100,
		AdaptabilityScore: math.Round(adaptabilityScore*100) / 100,
		FormalCount:       formal,
		CasualCount:       casual,
		EmpatheticCount:   empathetic,
	}

	score := 30.0
	score += formalityScore * 15.0
	score += empathyScore * 15.0
	score += authorityScore * 10.0
	score += neutralityScore * 10.0
	score += adaptabilityScore * 10.0

	score -= float64(casual) * 3.0

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

	var models []ModelTonality
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail TonalityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.FormalityScore += detail.FormalityScore
			totalDetail.EmpathyScore += detail.EmpathyScore
			totalDetail.AuthorityScore += detail.AuthorityScore
			totalDetail.NeutralityScore += detail.NeutralityScore
			totalDetail.AdaptabilityScore += detail.AdaptabilityScore
			totalDetail.FormalCount += detail.FormalCount
			totalDetail.CasualCount += detail.CasualCount
			totalDetail.EmpatheticCount += detail.EmpatheticCount

			if detail.CasualCount > 5 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.FormalityScore = math.Round(totalDetail.FormalityScore/n*100) / 100
		totalDetail.EmpathyScore = math.Round(totalDetail.EmpathyScore/n*100) / 100
		totalDetail.AuthorityScore = math.Round(totalDetail.AuthorityScore/n*100) / 100
		totalDetail.NeutralityScore = math.Round(totalDetail.NeutralityScore/n*100) / 100
		totalDetail.AdaptabilityScore = math.Round(totalDetail.AdaptabilityScore/n*100) / 100

		models = append(models, ModelTonality{
			Model:         mn,
			TonalityScore: math.Round(avgScore*10) / 10,
			Grade:         scoreToGrade(avgScore),
			Detail:        totalDetail,
			SampleCount:   len(entries),
			LabCount:      len(labs),
			Concerns:      totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].TonalityScore > models[j].TonalityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.TonalityScore
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
	b.WriteString("Response Tonality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No tonality data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.TonalityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "formality_score", m.Detail.FormalityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "empathy_score", m.Detail.EmpathyScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "authority_score", m.Detail.AuthorityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "neutrality_score", m.Detail.NeutralityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "adaptability_score", m.Detail.AdaptabilityScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "formal_count", m.Detail.FormalCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "casual_count", m.Detail.CasualCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "empathetic_count", m.Detail.EmpatheticCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
