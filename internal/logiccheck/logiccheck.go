package logiccheck

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type LogicDetail struct {
	ContradictionCues int `json:"contradiction_cues"`
	TautologyCues     int `json:"tautology_cues"`
	FallacyCues       int `json:"fallacy_cues"`
	CircularCues      int `json:"circular_cues"`
	NonSequiturCues   int `json:"non_sequitur_cues"`
	SoundReasonCues   int `json:"sound_reason_cues"`
}

type ModelLogic struct {
	Model       string      `json:"model"`
	LogicScore  float64     `json:"logic_score"`
	Grade       string      `json:"grade"`
	Detail      LogicDetail `json:"detail"`
	SampleCount int         `json:"sample_count"`
	LabCount    int         `json:"lab_count"`
	PoorLogic   int         `json:"poor_logic"`
}

type Report struct {
	Models      []ModelLogic `json:"models"`
	BestLogic   string       `json:"best_logic"`
	WorstLogic  string       `json:"worst_logic"`
	MeanScore   float64      `json:"mean_score"`
}

var contradictionPhrases = []string{
	"but also", "however it is also true that the opposite",
	"contradicts", "contradicting", "contradiction",
	"on one hand", "but then again",
	"this is true but also false",
	"both true and false", "cannot be both",
	"mutually exclusive yet", "inconsistent with",
	"conflicts with the previous", "negates the earlier",
}

var tautologyPhrases = []string{
	"by definition", "is always true",
	"true by nature", "self-evident truth",
	"goes without saying", "obvious truth",
	"tautological", "tautology",
	"redundantly", "repeating the same",
	"circular definition", "defined by itself",
}

var fallacyPhrases = []string{
	"straw man", "ad hominem", "red herring",
	"false dichotomy", "slippery slope",
	"appeal to authority", "appeal to emotion",
	"begging the question", "false equivalence",
	"hasty generalization", "post hoc",
	"no true scotsman", "bandwagon",
	"tu quoque", "genetic fallacy",
}

var circularPhrases = []string{
	"because it is", "the reason is that it is",
	"this proves itself", "self-referential proof",
	"circular reasoning", "circular argument",
	"proves its own conclusion", "assumes what it proves",
	"presupposes the conclusion", "begs the question",
}

var nonSequiturPhrases = []string{
	"non sequitur", "does not follow",
	"unrelated conclusion", "irrelevant reasoning",
	"leap of logic", "unjustified jump",
	"disconnected from", "no logical connection",
	"randomly concludes", "arbitrary conclusion",
}

var soundReasonPhrases = []string{
	"therefore", "thus", "consequently",
	"it follows that", "this implies",
	"logically", "by logical extension",
	"given the premises", "the evidence shows",
	"deductively", "inductively",
	"based on this reasoning", "if then",
	"valid inference", "sound argument",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(text string) (float64, LogicDetail) {
	contradictions := countPhrases(text, contradictionPhrases)
	tautologies := countPhrases(text, tautologyPhrases)
	fallacies := countPhrases(text, fallacyPhrases)
	circular := countPhrases(text, circularPhrases)
	nonSequitur := countPhrases(text, nonSequiturPhrases)
	soundReason := countPhrases(text, soundReasonPhrases)

	detail := LogicDetail{
		ContradictionCues: contradictions,
		TautologyCues:     tautologies,
		FallacyCues:       fallacies,
		CircularCues:      circular,
		NonSequiturCues:   nonSequitur,
		SoundReasonCues:   soundReason,
	}

	words := len(strings.Fields(text))
	if words == 0 {
		return 100, detail
	}

	flawSignals := float64(contradictions*4 + tautologies*2 + fallacies*3 + circular*3 + nonSequitur*3)
	soundSignals := float64(soundReason * 3)

	normalizer := math.Log2(float64(words) + 1)
	if normalizer < 1 {
		normalizer = 1
	}

	rawScore := (soundSignals - flawSignals) / normalizer * 10
	score := 80 + rawScore
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

	var models []ModelLogic
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail LogicDetail
		poorLogic := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := scoreResponse(e.Summary)
			totalScore += score
			totalDetail.ContradictionCues += detail.ContradictionCues
			totalDetail.TautologyCues += detail.TautologyCues
			totalDetail.FallacyCues += detail.FallacyCues
			totalDetail.CircularCues += detail.CircularCues
			totalDetail.NonSequiturCues += detail.NonSequiturCues
			totalDetail.SoundReasonCues += detail.SoundReasonCues

			if score < 50 {
				poorLogic++
			}
		}

		avgScore := totalScore / float64(len(entries))
		models = append(models, ModelLogic{
			Model:       mn,
			LogicScore:  math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
			PoorLogic:   poorLogic,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].LogicScore > models[j].LogicScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.LogicScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestLogic = models[0].Model
		r.WorstLogic = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Logic Check Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No logic data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best logic:           %s\n", r.BestLogic)
	fmt.Fprintf(&b, "  Worst logic:          %s\n\n", r.WorstLogic)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.LogicScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "contradiction_cues", m.Detail.ContradictionCues)
		fmt.Fprintf(&b, "    %-28s %d\n", "tautology_cues", m.Detail.TautologyCues)
		fmt.Fprintf(&b, "    %-28s %d\n", "fallacy_cues", m.Detail.FallacyCues)
		fmt.Fprintf(&b, "    %-28s %d\n", "circular_cues", m.Detail.CircularCues)
		fmt.Fprintf(&b, "    %-28s %d\n", "non_sequitur_cues", m.Detail.NonSequiturCues)
		fmt.Fprintf(&b, "    %-28s %d\n", "sound_reason_cues", m.Detail.SoundReasonCues)
		fmt.Fprintf(&b, "    Poor logic: %d\n", m.PoorLogic)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
