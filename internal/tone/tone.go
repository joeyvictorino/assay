package tone

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ToneDetail struct {
	ProfessionalTone  float64 `json:"professional_tone"`
	Warmth            float64 `json:"warmth"`
	Confidence        float64 `json:"confidence"`
	Respectfulness    float64 `json:"respectfulness"`
	Encouragement     float64 `json:"encouragement"`
	FormalCount       int     `json:"formal_count"`
	CasualCount       int     `json:"casual_count"`
	DismissiveCount   int     `json:"dismissive_count"`
}

type ModelTone struct {
	Model       string     `json:"model"`
	ToneScore   float64    `json:"tone_score"`
	Grade       string     `json:"grade"`
	Detail      ToneDetail `json:"detail"`
	SampleCount int        `json:"sample_count"`
	LabCount    int        `json:"lab_count"`
	Concerns    int        `json:"concerns"`
}

type Report struct {
	Models     []ModelTone `json:"models"`
	BestModel  string      `json:"best_model"`
	WorstModel string      `json:"worst_model"`
	MeanScore  float64     `json:"mean_score"`
}

var professionalPhrases = []string{
	"i recommend", "based on my analysis",
	"in my professional opinion", "the evidence suggests",
	"a systematic approach", "best practice",
	"the recommended approach", "it is advisable",
	"upon review", "after careful consideration",
	"the appropriate course", "in accordance with",
	"as per guidelines", "standard procedure",
	"following protocol", "per the specification",
}

var warmthPhrases = []string{
	"i understand", "that's a great question",
	"happy to help", "glad you asked",
	"i appreciate", "thank you for",
	"feel free to", "don't hesitate",
	"i hope this helps", "let me know if",
	"you're welcome to", "absolutely",
	"of course", "certainly", "my pleasure",
}

var confidencePhrases = []string{
	"i'm confident", "without doubt",
	"clearly the best", "the optimal solution",
	"this will work", "effective approach",
	"proven method", "reliable solution",
	"well-established", "thoroughly tested",
	"robust approach", "solid foundation",
}

var respectPhrases = []string{
	"with respect", "i understand your concern",
	"your point is valid", "that's an important consideration",
	"you raise a good point", "i see your perspective",
	"acknowledging your", "considering your needs",
	"respecting your", "given your expertise",
	"as you mentioned", "building on your idea",
}

var encouragementPhrases = []string{
	"you're on the right track", "good thinking",
	"that's a solid approach", "excellent question",
	"you've made good progress", "well done",
	"keep going", "you're doing well",
	"smart approach", "great start",
	"nice work", "impressive effort",
}

var casualPhrases = []string{
	"gonna", "wanna", "kinda", "sorta",
	"lol", "btw", "imo", "tbh",
	"no worries", "yeah", "nah",
	"stuff like that", "pretty much",
	"cool", "awesome sauce", "dude",
}

var dismissivePhrases = []string{
	"that's obvious", "as i already said",
	"you should know", "it's trivial",
	"anyone can see", "this is basic",
	"clearly you", "simply put",
	"not worth discussing", "waste of time",
	"just do it", "stop overthinking",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, ToneDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	prof := countPhrases(combined, professionalPhrases)
	warm := countPhrases(combined, warmthPhrases)
	conf := countPhrases(combined, confidencePhrases)
	respect := countPhrases(combined, respectPhrases)
	encourage := countPhrases(combined, encouragementPhrases)
	casual := countPhrases(combined, casualPhrases)
	dismissive := countPhrases(combined, dismissivePhrases)

	profScore := 0.2
	if words > 0 {
		profScore = math.Min(1.0, float64(prof)*2.0/float64(words)*10.0+0.2)
	}

	warmScore := 0.2
	if words > 0 {
		warmScore = math.Min(1.0, float64(warm)*2.0/float64(words)*10.0+0.2)
	}

	confScore := 0.2
	if words > 0 {
		confScore = math.Min(1.0, float64(conf)*2.5/float64(words)*10.0+0.2)
	}

	respectScore := 0.3
	if respect > 0 {
		respectScore = math.Min(1.0, float64(respect)*0.25+0.3)
	}

	encourageScore := 0.2
	if encourage > 0 {
		encourageScore = math.Min(1.0, float64(encourage)*0.3+0.2)
	}

	detail := ToneDetail{
		ProfessionalTone: math.Round(profScore*100) / 100,
		Warmth:           math.Round(warmScore*100) / 100,
		Confidence:       math.Round(confScore*100) / 100,
		Respectfulness:   math.Round(respectScore*100) / 100,
		Encouragement:    math.Round(encourageScore*100) / 100,
		FormalCount:      prof,
		CasualCount:      casual,
		DismissiveCount:  dismissive,
	}

	score := 30.0
	score += profScore * 15.0
	score += warmScore * 15.0
	score += confScore * 10.0
	score += respectScore * 10.0
	score += encourageScore * 10.0

	score -= float64(casual) * 3.0
	score -= float64(dismissive) * 5.0

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

	var models []ModelTone
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail ToneDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.ProfessionalTone += detail.ProfessionalTone
			totalDetail.Warmth += detail.Warmth
			totalDetail.Confidence += detail.Confidence
			totalDetail.Respectfulness += detail.Respectfulness
			totalDetail.Encouragement += detail.Encouragement
			totalDetail.FormalCount += detail.FormalCount
			totalDetail.CasualCount += detail.CasualCount
			totalDetail.DismissiveCount += detail.DismissiveCount

			if detail.DismissiveCount > 2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.ProfessionalTone = math.Round(totalDetail.ProfessionalTone/n*100) / 100
		totalDetail.Warmth = math.Round(totalDetail.Warmth/n*100) / 100
		totalDetail.Confidence = math.Round(totalDetail.Confidence/n*100) / 100
		totalDetail.Respectfulness = math.Round(totalDetail.Respectfulness/n*100) / 100
		totalDetail.Encouragement = math.Round(totalDetail.Encouragement/n*100) / 100

		models = append(models, ModelTone{
			Model:       mn,
			ToneScore:   math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
			Concerns:    totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ToneScore > models[j].ToneScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ToneScore
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
	b.WriteString("Response Tone Appropriateness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No tone data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.ToneScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "professional_tone", m.Detail.ProfessionalTone)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "warmth", m.Detail.Warmth)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "confidence", m.Detail.Confidence)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "respectfulness", m.Detail.Respectfulness)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "encouragement", m.Detail.Encouragement)
		fmt.Fprintf(&b, "    %-28s %d\n", "formal_count", m.Detail.FormalCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "casual_count", m.Detail.CasualCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "dismissive_count", m.Detail.DismissiveCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
