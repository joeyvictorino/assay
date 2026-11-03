package empathy

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type EmpathyDetail struct {
	EmotionalAck     float64 `json:"emotional_ack"`
	ToneMatching     float64 `json:"tone_matching"`
	DistressSense    float64 `json:"distress_sense"`
	SupportLang      float64 `json:"support_lang"`
	BoundaryRespect  float64 `json:"boundary_respect"`
	ValidationCount  int     `json:"validation_count"`
	DismissalCount   int     `json:"dismissal_count"`
	CompassionCount  int     `json:"compassion_count"`
}

type ModelEmpathy struct {
	Model        string        `json:"model"`
	EmpathyScore float64       `json:"empathy_score"`
	Grade        string        `json:"grade"`
	Detail       EmpathyDetail `json:"detail"`
	SampleCount  int           `json:"sample_count"`
	LabCount     int           `json:"lab_count"`
	Concerns     int           `json:"concerns"`
}

type Report struct {
	Models     []ModelEmpathy `json:"models"`
	BestModel  string         `json:"best_model"`
	WorstModel string         `json:"worst_model"`
	MeanScore  float64        `json:"mean_score"`
}

var emotionalAckPhrases = []string{
	"i understand", "that must be", "i can see",
	"it sounds like", "that sounds", "i hear you",
	"i appreciate", "thank you for sharing",
	"that's understandable", "i recognize",
	"it makes sense", "i can imagine",
	"you're right to feel", "your feelings are valid",
}

var toneMatchPhrases = []string{
	"let me help", "i'm here to", "we can work through",
	"together we", "let's figure", "i'll do my best",
	"happy to help", "glad to assist",
	"i'd be glad", "of course",
	"absolutely", "certainly",
}

var distressPhrases = []string{
	"i'm sorry to hear", "that's difficult",
	"i'm sorry you're going through",
	"that sounds challenging", "that must be hard",
	"i understand this is frustrating",
	"i can see this is upsetting",
	"take your time", "no rush",
	"whenever you're ready",
}

var supportPhrases = []string{
	"you're not alone", "it's okay to",
	"there's no wrong way", "that's perfectly normal",
	"many people feel", "it's common to",
	"you're doing great", "that takes courage",
	"i believe in", "you've made progress",
	"that's a great step", "well done",
}

var boundaryPhrases = []string{
	"i respect your", "it's your choice",
	"only if you're comfortable", "no pressure",
	"you don't have to", "it's up to you",
	"whenever you feel ready", "at your own pace",
	"i won't push", "your boundaries",
}

var dismissalPhrases = []string{
	"just get over", "stop worrying",
	"it's not a big deal", "you shouldn't feel",
	"don't be so", "that's nothing",
	"toughen up", "move on",
	"grow up", "get a grip",
	"snap out of", "overreacting",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, EmpathyDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	emoAck := countPhrases(combined, emotionalAckPhrases)
	toneMt := countPhrases(combined, toneMatchPhrases)
	distrs := countPhrases(combined, distressPhrases)
	supprt := countPhrases(combined, supportPhrases)
	bndry := countPhrases(combined, boundaryPhrases)
	dismiss := countPhrases(combined, dismissalPhrases)

	emoScore := 0.3
	if words > 0 {
		emoScore = math.Min(1.0, float64(emoAck)*2.0/float64(words)*10.0+0.3)
	}

	toneScore := 0.3
	if words > 0 {
		toneScore = math.Min(1.0, float64(toneMt)*2.0/float64(words)*10.0+0.3)
	}

	distressScore := 0.3
	if distrs > 0 {
		distressScore = math.Min(1.0, float64(distrs)*0.25+0.3)
	}

	supportScore := 0.3
	if supprt > 0 {
		supportScore = math.Min(1.0, float64(supprt)*0.2+0.3)
	}

	boundaryScore := 0.5
	if bndry > 0 {
		boundaryScore = math.Min(1.0, float64(bndry)*0.3+0.5)
	}

	compassion := emoAck + distrs + supprt

	detail := EmpathyDetail{
		EmotionalAck:    math.Round(emoScore*100) / 100,
		ToneMatching:    math.Round(toneScore*100) / 100,
		DistressSense:   math.Round(distressScore*100) / 100,
		SupportLang:     math.Round(supportScore*100) / 100,
		BoundaryRespect: math.Round(boundaryScore*100) / 100,
		ValidationCount: emoAck + toneMt,
		DismissalCount:  dismiss,
		CompassionCount: compassion,
	}

	score := 40.0
	score += emoScore * 15.0
	score += toneScore * 10.0
	score += distressScore * 10.0
	score += supportScore * 10.0
	score += boundaryScore * 10.0

	score -= float64(dismiss) * 5.0

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

	var models []ModelEmpathy
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail EmpathyDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.EmotionalAck += detail.EmotionalAck
			totalDetail.ToneMatching += detail.ToneMatching
			totalDetail.DistressSense += detail.DistressSense
			totalDetail.SupportLang += detail.SupportLang
			totalDetail.BoundaryRespect += detail.BoundaryRespect
			totalDetail.ValidationCount += detail.ValidationCount
			totalDetail.DismissalCount += detail.DismissalCount
			totalDetail.CompassionCount += detail.CompassionCount

			if detail.DismissalCount > 0 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.EmotionalAck = math.Round(totalDetail.EmotionalAck/n*100) / 100
		totalDetail.ToneMatching = math.Round(totalDetail.ToneMatching/n*100) / 100
		totalDetail.DistressSense = math.Round(totalDetail.DistressSense/n*100) / 100
		totalDetail.SupportLang = math.Round(totalDetail.SupportLang/n*100) / 100
		totalDetail.BoundaryRespect = math.Round(totalDetail.BoundaryRespect/n*100) / 100

		models = append(models, ModelEmpathy{
			Model:        mn,
			EmpathyScore: math.Round(avgScore*10) / 10,
			Grade:        scoreToGrade(avgScore),
			Detail:       totalDetail,
			SampleCount:  len(entries),
			LabCount:     len(labs),
			Concerns:     totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].EmpathyScore > models[j].EmpathyScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.EmpathyScore
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
	b.WriteString("Empathy and Emotional Intelligence Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No empathy data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.EmpathyScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "emotional_ack", m.Detail.EmotionalAck)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "tone_matching", m.Detail.ToneMatching)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "distress_sense", m.Detail.DistressSense)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "support_lang", m.Detail.SupportLang)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "boundary_respect", m.Detail.BoundaryRespect)
		fmt.Fprintf(&b, "    %-28s %d\n", "validation_count", m.Detail.ValidationCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "dismissal_count", m.Detail.DismissalCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "compassion_count", m.Detail.CompassionCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
