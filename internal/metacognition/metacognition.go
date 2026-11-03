package metacognition

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type MetacognitionDetail struct {
	SelfAwareness       float64 `json:"self_awareness"`
	ReasoningTransparency float64 `json:"reasoning_transparency"`
	UncertaintyCalibration float64 `json:"uncertainty_calibration"`
	LimitAcknowledgment float64 `json:"limit_acknowledgment"`
	ThinkingProcess     float64 `json:"thinking_process"`
	ReflectiveCount     int     `json:"reflective_count"`
	OverconfidentCount  int     `json:"overconfident_count"`
	TransparentCount    int     `json:"transparent_count"`
}

type ModelMetacognition struct {
	Model              string              `json:"model"`
	MetacognitionScore float64             `json:"metacognition_score"`
	Grade              string              `json:"grade"`
	Detail             MetacognitionDetail `json:"detail"`
	SampleCount        int                 `json:"sample_count"`
	LabCount           int                 `json:"lab_count"`
	Concerns           int                 `json:"concerns"`
}

type Report struct {
	Models     []ModelMetacognition `json:"models"`
	BestModel  string               `json:"best_model"`
	WorstModel string               `json:"worst_model"`
	MeanScore  float64              `json:"mean_score"`
}

var selfAwarePhrases = []string{
	"i should note", "i want to be transparent",
	"my reasoning here is", "i'm approaching this by",
	"let me think through", "stepping back",
	"on reflection", "reconsidering",
	"i realize", "upon further thought",
	"my initial instinct", "after more consideration",
	"i should clarify", "to be honest about my reasoning",
	"my confidence level", "i'm reasoning from",
}

var transparencyPhrases = []string{
	"my reasoning is", "the logic here is",
	"i'm basing this on", "the evidence suggests",
	"here's why i think", "my thought process",
	"the key factors are", "i'm weighing",
	"considering the trade-offs", "the reasoning chain",
	"step by step", "working through this",
	"to break this down", "the underlying logic",
	"connecting these points", "following this thread",
}

var uncertaintyPhrases = []string{
	"i'm not certain", "i could be wrong",
	"this is my best understanding", "take this with caution",
	"i'm less confident about", "this might not be accurate",
	"i have limited knowledge", "this is uncertain",
	"subject to change", "needs verification",
	"approximately", "roughly speaking",
	"as far as i can tell", "to the best of my knowledge",
}

var limitPhrases = []string{
	"i don't have access to", "beyond my knowledge",
	"i can't verify", "outside my expertise",
	"i'm not qualified to", "this exceeds my capability",
	"you should consult", "a specialist would know",
	"real-time data is unavailable", "my training data",
	"i lack the context", "i cannot confirm",
	"i'm unable to determine", "this requires human judgment",
}

var thinkingPhrases = []string{
	"first, let me consider", "there are several factors",
	"weighing the options", "comparing alternatives",
	"the main consideration", "prioritizing",
	"the most important factor", "a key insight",
	"what stands out", "the critical question",
	"this leads me to", "therefore",
	"this implies", "from this we can infer",
}

var overconfidentPhrases = []string{
	"without a doubt", "this is absolutely",
	"i guarantee", "there is no question",
	"this is definitely the only", "you must",
	"the only correct answer", "undeniable",
	"beyond any dispute", "with complete certainty",
	"i'm 100% sure", "no other possibility",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, MetacognitionDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	selfAware := countPhrases(combined, selfAwarePhrases)
	transparent := countPhrases(combined, transparencyPhrases)
	uncertain := countPhrases(combined, uncertaintyPhrases)
	limits := countPhrases(combined, limitPhrases)
	thinking := countPhrases(combined, thinkingPhrases)
	overconfident := countPhrases(combined, overconfidentPhrases)

	reflective := selfAware + transparent + uncertain + limits

	awarenessScore := 0.2
	if words > 0 {
		awarenessScore = math.Min(1.0, float64(selfAware)*2.0/float64(words)*10.0+0.2)
	}

	transparencyScore := 0.2
	if words > 0 {
		transparencyScore = math.Min(1.0, float64(transparent)*2.0/float64(words)*10.0+0.2)
	}

	calibrationScore := 0.3
	if uncertain > 0 || limits > 0 {
		calibrationScore = math.Min(1.0, float64(uncertain+limits)*0.15+0.3)
	}

	limitScore := 0.2
	if words > 0 {
		limitScore = math.Min(1.0, float64(limits)*2.0/float64(words)*10.0+0.2)
	}

	thinkingScore := 0.2
	if words > 0 {
		thinkingScore = math.Min(1.0, float64(thinking)*2.0/float64(words)*10.0+0.2)
	}

	detail := MetacognitionDetail{
		SelfAwareness:          math.Round(awarenessScore*100) / 100,
		ReasoningTransparency:  math.Round(transparencyScore*100) / 100,
		UncertaintyCalibration: math.Round(calibrationScore*100) / 100,
		LimitAcknowledgment:    math.Round(limitScore*100) / 100,
		ThinkingProcess:        math.Round(thinkingScore*100) / 100,
		ReflectiveCount:        reflective,
		OverconfidentCount:     overconfident,
		TransparentCount:       transparent,
	}

	score := 30.0
	score += awarenessScore * 15.0
	score += transparencyScore * 15.0
	score += calibrationScore * 10.0
	score += limitScore * 10.0
	score += thinkingScore * 10.0

	score -= float64(overconfident) * 5.0
	score += float64(reflective) * 0.5

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

	var models []ModelMetacognition
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail MetacognitionDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.SelfAwareness += detail.SelfAwareness
			totalDetail.ReasoningTransparency += detail.ReasoningTransparency
			totalDetail.UncertaintyCalibration += detail.UncertaintyCalibration
			totalDetail.LimitAcknowledgment += detail.LimitAcknowledgment
			totalDetail.ThinkingProcess += detail.ThinkingProcess
			totalDetail.ReflectiveCount += detail.ReflectiveCount
			totalDetail.OverconfidentCount += detail.OverconfidentCount
			totalDetail.TransparentCount += detail.TransparentCount

			if detail.OverconfidentCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.SelfAwareness = math.Round(totalDetail.SelfAwareness/n*100) / 100
		totalDetail.ReasoningTransparency = math.Round(totalDetail.ReasoningTransparency/n*100) / 100
		totalDetail.UncertaintyCalibration = math.Round(totalDetail.UncertaintyCalibration/n*100) / 100
		totalDetail.LimitAcknowledgment = math.Round(totalDetail.LimitAcknowledgment/n*100) / 100
		totalDetail.ThinkingProcess = math.Round(totalDetail.ThinkingProcess/n*100) / 100

		models = append(models, ModelMetacognition{
			Model:              mn,
			MetacognitionScore: math.Round(avgScore*10) / 10,
			Grade:              scoreToGrade(avgScore),
			Detail:             totalDetail,
			SampleCount:        len(entries),
			LabCount:           len(labs),
			Concerns:           totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].MetacognitionScore > models[j].MetacognitionScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.MetacognitionScore
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
	b.WriteString("Response Metacognition Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No metacognition data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.MetacognitionScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "self_awareness", m.Detail.SelfAwareness)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "reasoning_transparency", m.Detail.ReasoningTransparency)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "uncertainty_calibration", m.Detail.UncertaintyCalibration)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "limit_acknowledgment", m.Detail.LimitAcknowledgment)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "thinking_process", m.Detail.ThinkingProcess)
		fmt.Fprintf(&b, "    %-28s %d\n", "reflective_count", m.Detail.ReflectiveCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "overconfident_count", m.Detail.OverconfidentCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "transparent_count", m.Detail.TransparentCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
