package objectivity

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ObjectivityDetail struct {
	ImpartialTone      float64 `json:"impartial_tone"`
	EvidenceBased      float64 `json:"evidence_based"`
	BalancedViewpoint  float64 `json:"balanced_viewpoint"`
	NeutralLanguage    float64 `json:"neutral_language"`
	SourceAttribution  float64 `json:"source_attribution"`
	NeutralCount       int     `json:"neutral_count"`
	BiasCount          int     `json:"bias_count"`
	AttributionCount   int     `json:"attribution_count"`
}

type ModelObjectivity struct {
	Model            string            `json:"model"`
	ObjectivityScore float64           `json:"objectivity_score"`
	Grade            string            `json:"grade"`
	Detail           ObjectivityDetail `json:"detail"`
	SampleCount      int               `json:"sample_count"`
	LabCount         int               `json:"lab_count"`
	Concerns         int               `json:"concerns"`
}

type Report struct {
	Models     []ModelObjectivity `json:"models"`
	BestModel  string             `json:"best_model"`
	WorstModel string             `json:"worst_model"`
	MeanScore  float64            `json:"mean_score"`
}

var impartialPhrases = []string{
	"objectively", "from a neutral standpoint",
	"without bias", "impartially", "dispassionately",
	"based on the facts", "factually speaking",
	"the data shows", "evidence indicates",
	"analysis reveals", "observations suggest",
	"measured approach", "balanced assessment",
	"fair evaluation", "unbiased view",
}

var evidenceBasedPhrases = []string{
	"according to", "research indicates", "studies show",
	"data demonstrates", "evidence suggests", "based on findings",
	"empirical data", "statistically", "measured by",
	"quantified as", "peer-reviewed", "documented in",
	"verified through", "confirmed by", "supported by data",
}

var balancedPhrases = []string{
	"on one hand", "on the other hand", "conversely",
	"proponents argue", "critics point out",
	"supporters note", "opponents contend",
	"from one perspective", "from another perspective",
	"while some believe", "others argue",
	"both sides", "multiple viewpoints",
	"competing interpretations", "alternative view",
}

var neutralPhrases = []string{
	"it appears", "it seems", "one could argue",
	"it is possible", "there is evidence",
	"some suggest", "it may be",
	"the situation indicates", "given the context",
	"from this standpoint", "taking into account",
	"considering the evidence", "upon examination",
}

var attributionPhrases = []string{
	"according to the source", "as reported by",
	"the author states", "researchers found",
	"the study concludes", "experts note",
	"analysts observe", "the report indicates",
	"as documented", "per the findings",
	"the literature suggests", "as cited in",
}

var biasPhrases = []string{
	"i believe", "in my opinion", "i feel that",
	"i think", "personally", "my view is",
	"i am convinced", "i strongly believe",
	"without a doubt in my mind", "i am certain",
	"my experience tells me", "i prefer",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, ObjectivityDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	impartial := countPhrases(combined, impartialPhrases)
	evidBased := countPhrases(combined, evidenceBasedPhrases)
	balanced := countPhrases(combined, balancedPhrases)
	neutral := countPhrases(combined, neutralPhrases)
	attribution := countPhrases(combined, attributionPhrases)
	bias := countPhrases(combined, biasPhrases)

	impartialScore := 0.2
	if words > 0 {
		impartialScore = math.Min(1.0, float64(impartial)*2.0/float64(words)*10.0+0.2)
	}

	evidScore := 0.2
	if words > 0 {
		evidScore = math.Min(1.0, float64(evidBased)*2.0/float64(words)*10.0+0.2)
	}

	balanceScore := 0.2
	if words > 0 {
		balanceScore = math.Min(1.0, float64(balanced)*2.5/float64(words)*10.0+0.2)
	}

	neutralScore := 0.3
	if neutral > 0 {
		neutralScore = math.Min(1.0, float64(neutral)*0.25+0.3)
	}

	attrScore := 0.2
	if attribution > 0 {
		attrScore = math.Min(1.0, float64(attribution)*0.3+0.2)
	}

	detail := ObjectivityDetail{
		ImpartialTone:     math.Round(impartialScore*100) / 100,
		EvidenceBased:     math.Round(evidScore*100) / 100,
		BalancedViewpoint: math.Round(balanceScore*100) / 100,
		NeutralLanguage:   math.Round(neutralScore*100) / 100,
		SourceAttribution: math.Round(attrScore*100) / 100,
		NeutralCount:      neutral,
		BiasCount:         bias,
		AttributionCount:  attribution,
	}

	score := 30.0
	score += impartialScore * 15.0
	score += evidScore * 15.0
	score += balanceScore * 15.0
	score += neutralScore * 10.0
	score += attrScore * 10.0

	score -= float64(bias) * 4.0

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

	var models []ModelObjectivity
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail ObjectivityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ImpartialTone += detail.ImpartialTone
			totalDetail.EvidenceBased += detail.EvidenceBased
			totalDetail.BalancedViewpoint += detail.BalancedViewpoint
			totalDetail.NeutralLanguage += detail.NeutralLanguage
			totalDetail.SourceAttribution += detail.SourceAttribution
			totalDetail.NeutralCount += detail.NeutralCount
			totalDetail.BiasCount += detail.BiasCount
			totalDetail.AttributionCount += detail.AttributionCount

			if detail.BiasCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ImpartialTone = math.Round(totalDetail.ImpartialTone/n*100) / 100
		totalDetail.EvidenceBased = math.Round(totalDetail.EvidenceBased/n*100) / 100
		totalDetail.BalancedViewpoint = math.Round(totalDetail.BalancedViewpoint/n*100) / 100
		totalDetail.NeutralLanguage = math.Round(totalDetail.NeutralLanguage/n*100) / 100
		totalDetail.SourceAttribution = math.Round(totalDetail.SourceAttribution/n*100) / 100

		models = append(models, ModelObjectivity{
			Model:            mn,
			ObjectivityScore: math.Round(avgScore*10) / 10,
			Grade:            scoreToGrade(avgScore),
			Detail:           totalDetail,
			SampleCount:      len(entries),
			LabCount:         len(labs),
			Concerns:         totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ObjectivityScore > models[j].ObjectivityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ObjectivityScore
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
	b.WriteString("Response Objectivity Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No objectivity data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.ObjectivityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "impartial_tone", m.Detail.ImpartialTone)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "evidence_based", m.Detail.EvidenceBased)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "balanced_viewpoint", m.Detail.BalancedViewpoint)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "neutral_language", m.Detail.NeutralLanguage)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "source_attribution", m.Detail.SourceAttribution)
		fmt.Fprintf(&b, "    %-28s %d\n", "neutral_count", m.Detail.NeutralCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "bias_count", m.Detail.BiasCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "attribution_count", m.Detail.AttributionCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
