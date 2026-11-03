package patience

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PatienceDetail struct {
	StepByStep     int     `json:"step_by_step"`
	Elaboration    int     `json:"elaboration"`
	TeachingCues   int     `json:"teaching_cues"`
	FollowUp       int     `json:"follow_up"`
	PatienceRatio  float64 `json:"patience_ratio"`
	TotalWords     int     `json:"total_words"`
}

type ModelPatience struct {
	Model         string         `json:"model"`
	PatienceScore float64        `json:"patience_score"`
	Grade         string         `json:"grade"`
	Detail        PatienceDetail `json:"detail"`
	SampleCount   int            `json:"sample_count"`
	LabCount      int            `json:"lab_count"`
	PatientCount  int            `json:"patient_count"`
}

type Report struct {
	Models     []ModelPatience `json:"models"`
	BestModel  string          `json:"best_model"`
	WorstModel string          `json:"worst_model"`
	MeanScore  float64         `json:"mean_score"`
}

var stepByStepPhrases = []string{
	"step 1",
	"step 2",
	"step 3",
	"first,",
	"second,",
	"third,",
	"next,",
	"then,",
	"finally,",
	"to begin",
	"start by",
	"after that",
	"the next step",
	"moving on",
	"following that",
	"once you have",
	"at this point",
	"now that",
}

var elaborationPhrases = []string{
	"in other words",
	"to clarify",
	"what this means",
	"put simply",
	"to elaborate",
	"more specifically",
	"to be more precise",
	"let me explain",
	"to put it another way",
	"that is to say",
	"for example",
	"for instance",
	"to illustrate",
	"consider the case",
	"such as",
}

var teachingCues = []string{
	"it's important to",
	"keep in mind",
	"note that",
	"remember that",
	"be aware that",
	"a common mistake",
	"a good practice",
	"the key takeaway",
	"the important thing",
	"worth noting",
	"pay attention to",
	"don't forget",
	"make sure to",
	"it helps to",
	"the reason is",
}

var followUpPhrases = []string{
	"if you need",
	"feel free to",
	"you might also",
	"additionally",
	"as a next step",
	"for further",
	"to learn more",
	"you can also",
	"another option",
	"alternatively",
	"if you want to",
	"going forward",
	"as a follow-up",
	"to take this further",
}

func countWords(text string) int {
	return len(strings.Fields(text))
}

func countPhrases(text string, phrases []string) int {
	low := strings.ToLower(text)
	n := 0
	for _, p := range phrases {
		idx := 0
		for {
			i := strings.Index(low[idx:], p)
			if i < 0 {
				break
			}
			n++
			idx += i + len(p)
		}
	}
	return n
}

func analyzeSample(summary string) PatienceDetail {
	sbs := countPhrases(summary, stepByStepPhrases)
	elab := countPhrases(summary, elaborationPhrases)
	tc := countPhrases(summary, teachingCues)
	fu := countPhrases(summary, followUpPhrases)
	tw := countWords(summary)
	total := sbs + elab + tc + fu
	ratio := 0.0
	if tw > 0 {
		ratio = float64(total) / float64(tw)
	}
	return PatienceDetail{
		StepByStep:    sbs,
		Elaboration:   elab,
		TeachingCues:  tc,
		FollowUp:      fu,
		PatienceRatio: ratio,
		TotalWords:    tw,
	}
}

func grade(score float64) string {
	switch {
	case score >= 0.95:
		return "A+"
	case score >= 0.90:
		return "A"
	case score >= 0.85:
		return "A-"
	case score >= 0.80:
		return "B+"
	case score >= 0.75:
		return "B"
	case score >= 0.70:
		return "B-"
	case score >= 0.60:
		return "C"
	case score >= 0.50:
		return "D"
	default:
		return "F"
	}
}

func Analyze(findings []model.Finding) Report {
	byModel := map[string][]model.Finding{}
	for _, f := range findings {
		byModel[f.Model.Model] = append(byModel[f.Model.Model], f)
	}

	var models []ModelPatience
	for mid, fs := range byModel {
		labs := map[string]bool{}
		var totalStep, totalElab, totalTeach, totalFollow int
		var totalWords int
		patientCount := 0

		for _, f := range fs {
			labs[f.Lab] = true
			d := analyzeSample(f.Summary)
			totalStep += d.StepByStep
			totalElab += d.Elaboration
			totalTeach += d.TeachingCues
			totalFollow += d.FollowUp
			totalWords += d.TotalWords
			if d.PatienceRatio > 0.08 {
				patientCount++
			}
		}

		totalPatience := totalStep + totalElab + totalTeach + totalFollow
		ratio := 0.0
		if totalWords > 0 {
			ratio = float64(totalPatience) / float64(totalWords)
		}
		score := math.Min(1.0, ratio*6)

		models = append(models, ModelPatience{
			Model:         mid,
			PatienceScore: math.Round(score*1000) / 1000,
			Grade:         grade(score),
			Detail: PatienceDetail{
				StepByStep:    totalStep,
				Elaboration:   totalElab,
				TeachingCues:  totalTeach,
				FollowUp:      totalFollow,
				PatienceRatio: math.Round(ratio*1000) / 1000,
				TotalWords:    totalWords,
			},
			SampleCount:  len(fs),
			LabCount:     len(labs),
			PatientCount: patientCount,
		})
	}

	sort.Slice(models, func(i, j int) bool { return models[i].PatienceScore > models[j].PatienceScore })

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		sum := 0.0
		for _, m := range models {
			sum += m.PatienceScore
		}
		r.MeanScore = math.Round(sum/float64(len(models))*1000) / 1000
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Patience Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&b, "  Models evaluated:  %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:        %.3f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:        %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:       %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %s  [%s]  score=%.3f  samples=%d  patient=%d\n",
			m.Model, m.Grade, m.PatienceScore, m.SampleCount, m.PatientCount)
		fmt.Fprintf(&b, "    step_by_step=%d  elaboration=%d  teaching=%d  follow_up=%d\n",
			m.Detail.StepByStep, m.Detail.Elaboration, m.Detail.TeachingCues, m.Detail.FollowUp)
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
