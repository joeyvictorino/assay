package inferential

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type InferentialDetail struct {
	DeductiveStrength float64 `json:"deductive_strength"`
	InductiveQuality  float64 `json:"inductive_quality"`
	AbductiveSkill    float64 `json:"abductive_skill"`
	EvidenceUse       float64 `json:"evidence_use"`
	ConclusionValidity float64 `json:"conclusion_validity"`
	InferenceCount    int     `json:"inference_count"`
	FallacyCount      int     `json:"fallacy_count"`
	HedgeCount        int     `json:"hedge_count"`
}

type ModelInferential struct {
	Model            string            `json:"model"`
	InferentialScore float64           `json:"inferential_score"`
	Grade            string            `json:"grade"`
	Detail           InferentialDetail `json:"detail"`
	SampleCount      int               `json:"sample_count"`
	LabCount         int               `json:"lab_count"`
	Concerns         int               `json:"concerns"`
}

type Report struct {
	Models     []ModelInferential `json:"models"`
	BestModel  string             `json:"best_model"`
	WorstModel string             `json:"worst_model"`
	MeanScore  float64            `json:"mean_score"`
}

var deductivePhrases = []string{
	"therefore", "it follows that", "consequently",
	"thus", "hence", "logically",
	"by definition", "necessarily", "must be",
	"if and only if", "given that", "since",
	"it is certain that", "we can conclude", "this proves",
}

var inductivePhrases = []string{
	"based on the evidence", "the data suggests",
	"the pattern indicates", "observations show",
	"in most cases", "typically this means",
	"the trend suggests", "empirically",
	"from the sample", "statistically",
	"the evidence points to", "we can infer",
	"based on these examples", "the general rule",
	"extrapolating from",
}

var abductivePhrases = []string{
	"the best explanation", "most likely",
	"this would explain", "the simplest explanation",
	"a plausible cause", "probably because",
	"the evidence is consistent with", "hypothesis",
	"one possible explanation", "this accounts for",
	"the most reasonable interpretation", "suggests that",
	"could be attributed to", "the underlying cause",
	"inferring from the symptoms",
}

var evidencePhrases = []string{
	"as evidenced by", "supported by",
	"the data shows", "according to",
	"research indicates", "studies show",
	"the findings demonstrate", "corroborated by",
	"verified through", "measured by",
	"documented in", "confirmed by",
	"peer-reviewed", "reproducible results",
	"empirical evidence",
}

var fallacyPhrases = []string{
	"everyone knows", "obviously",
	"it goes without saying", "common sense dictates",
	"you can't deny", "clearly this proves",
	"no one would disagree", "it's self-evident",
	"only a fool would", "the slippery slope",
	"correlation implies causation", "since the beginning",
}

var hedgePhrases = []string{
	"it might be", "perhaps", "possibly",
	"i'm not entirely sure", "it could be argued",
	"one might think", "it seems possible",
	"there's a chance", "arguably",
	"it's debatable whether", "some might say",
	"conceivably",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, InferentialDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	deductive := countPhrases(combined, deductivePhrases)
	inductive := countPhrases(combined, inductivePhrases)
	abductive := countPhrases(combined, abductivePhrases)
	evidenceUse := countPhrases(combined, evidencePhrases)
	fallacy := countPhrases(combined, fallacyPhrases)
	hedge := countPhrases(combined, hedgePhrases)

	inferenceTotal := deductive + inductive + abductive

	dedScore := 0.2
	if words > 0 {
		dedScore = math.Min(1.0, float64(deductive)*2.0/float64(words)*10.0+0.2)
	}

	indScore := 0.2
	if words > 0 {
		indScore = math.Min(1.0, float64(inductive)*2.0/float64(words)*10.0+0.2)
	}

	abdScore := 0.2
	if words > 0 {
		abdScore = math.Min(1.0, float64(abductive)*2.0/float64(words)*10.0+0.2)
	}

	eviScore := 0.2
	if evidenceUse > 0 {
		eviScore = math.Min(1.0, float64(evidenceUse)*0.12+0.2)
	}

	concScore := 0.3
	if inferenceTotal > 0 && fallacy == 0 {
		concScore = math.Min(1.0, 0.4+float64(inferenceTotal)*0.06)
	} else if inferenceTotal > 0 {
		concScore = 0.35
	}

	detail := InferentialDetail{
		DeductiveStrength:  math.Round(dedScore*100) / 100,
		InductiveQuality:   math.Round(indScore*100) / 100,
		AbductiveSkill:     math.Round(abdScore*100) / 100,
		EvidenceUse:        math.Round(eviScore*100) / 100,
		ConclusionValidity: math.Round(concScore*100) / 100,
		InferenceCount:     inferenceTotal,
		FallacyCount:       fallacy,
		HedgeCount:         hedge,
	}

	score := 30.0
	score += dedScore * 14.0
	score += indScore * 14.0
	score += abdScore * 10.0
	score += eviScore * 10.0
	score += concScore * 12.0

	score -= float64(fallacy) * 5.0
	score -= float64(hedge) * 2.0

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

	var models []ModelInferential
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail InferentialDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.DeductiveStrength += detail.DeductiveStrength
			totalDetail.InductiveQuality += detail.InductiveQuality
			totalDetail.AbductiveSkill += detail.AbductiveSkill
			totalDetail.EvidenceUse += detail.EvidenceUse
			totalDetail.ConclusionValidity += detail.ConclusionValidity
			totalDetail.InferenceCount += detail.InferenceCount
			totalDetail.FallacyCount += detail.FallacyCount
			totalDetail.HedgeCount += detail.HedgeCount

			if detail.FallacyCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.DeductiveStrength = math.Round(totalDetail.DeductiveStrength/n*100) / 100
		totalDetail.InductiveQuality = math.Round(totalDetail.InductiveQuality/n*100) / 100
		totalDetail.AbductiveSkill = math.Round(totalDetail.AbductiveSkill/n*100) / 100
		totalDetail.EvidenceUse = math.Round(totalDetail.EvidenceUse/n*100) / 100
		totalDetail.ConclusionValidity = math.Round(totalDetail.ConclusionValidity/n*100) / 100

		models = append(models, ModelInferential{
			Model:            mn,
			InferentialScore: math.Round(avgScore*10) / 10,
			Grade:            scoreToGrade(avgScore),
			Detail:           totalDetail,
			SampleCount:      len(entries),
			LabCount:         len(labs),
			Concerns:         totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].InferentialScore > models[j].InferentialScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.InferentialScore
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
	b.WriteString("Inferential Reasoning Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No inferential data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.InferentialScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "deductive_strength", m.Detail.DeductiveStrength)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "inductive_quality", m.Detail.InductiveQuality)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "abductive_skill", m.Detail.AbductiveSkill)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "evidence_use", m.Detail.EvidenceUse)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "conclusion_validity", m.Detail.ConclusionValidity)
		fmt.Fprintf(&b, "    %-28s %d\n", "inference_count", m.Detail.InferenceCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "fallacy_count", m.Detail.FallacyCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "hedge_count", m.Detail.HedgeCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
