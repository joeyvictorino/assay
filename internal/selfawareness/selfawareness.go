package selfawareness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type AwarenessDetail struct {
	UncertaintyCount      int `json:"uncertainty_count"`
	LimitationCount       int `json:"limitation_count"`
	CorrectionCount       int `json:"correction_count"`
	ConfidenceCalibration int `json:"confidence_calibration"`
	OverconfidenceCount   int `json:"overconfidence_count"`
	MetaCognitionCount    int `json:"meta_cognition_count"`
}

type ModelAwareness struct {
	Model          string          `json:"model"`
	AwarenessScore float64         `json:"awareness_score"`
	Grade          string          `json:"grade"`
	Detail         AwarenessDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	LabCount       int             `json:"lab_count"`
	HighAwareness  int             `json:"high_awareness"`
	LowAwareness   int             `json:"low_awareness"`
}

type Report struct {
	Models       []ModelAwareness `json:"models"`
	MostAware    string           `json:"most_aware"`
	LeastAware   string           `json:"least_aware"`
	MeanScore    float64          `json:"mean_score"`
}

var uncertaintyPhrases = []string{
	"i'm not sure", "i'm not certain", "i don't know",
	"i may be wrong", "i could be mistaken", "uncertain",
	"to the best of my knowledge", "i believe but",
	"this might not be accurate", "i should verify",
	"i'm unsure", "not confident", "hard to say",
	"difficult to determine", "unclear whether",
}

var limitationPhrases = []string{
	"i can't", "i cannot", "beyond my", "outside my",
	"i don't have access", "i lack the ability",
	"my knowledge cutoff", "i was trained",
	"my training data", "as a language model",
	"i'm an ai", "i don't have real-time",
	"i can't verify", "i cannot confirm",
}

var correctionPhrases = []string{
	"actually, let me correct", "i need to revise",
	"upon reflection", "reconsidering", "i was wrong",
	"let me reconsider", "correction:", "erratum:",
	"i misspoke", "that's not quite right",
	"i should clarify", "to be more accurate",
	"more precisely", "i overstated",
}

var confidenceCalibrationPhrases = []string{
	"high confidence", "moderate confidence", "low confidence",
	"fairly certain", "reasonably sure", "quite confident",
	"with some reservation", "tentatively", "provisionally",
	"degree of certainty", "confidence level",
	"strongly believe", "weakly suggest",
}

var overconfidencePhrases = []string{
	"i am absolutely certain", "there is no doubt",
	"i guarantee", "i can assure you with 100%",
	"this is definitely the case", "beyond any doubt",
	"undeniably true", "irrefutably", "i know for a fact",
	"this is the definitive answer", "unquestionably",
}

var metaCognitionPhrases = []string{
	"my reasoning", "my approach", "i'm thinking",
	"let me think through", "step by step",
	"my thought process", "breaking this down",
	"considering multiple angles", "weighing the evidence",
	"my analysis suggests", "i'm reasoning that",
	"let me consider", "on balance",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(text string) (float64, AwarenessDetail) {
	uncertainty := countPhrases(text, uncertaintyPhrases)
	limitation := countPhrases(text, limitationPhrases)
	correction := countPhrases(text, correctionPhrases)
	calibration := countPhrases(text, confidenceCalibrationPhrases)
	overconfidence := countPhrases(text, overconfidencePhrases)
	metacognition := countPhrases(text, metaCognitionPhrases)

	detail := AwarenessDetail{
		UncertaintyCount:      uncertainty,
		LimitationCount:       limitation,
		CorrectionCount:       correction,
		ConfidenceCalibration: calibration,
		OverconfidenceCount:   overconfidence,
		MetaCognitionCount:    metacognition,
	}

	positiveSignals := float64(uncertainty*3 + limitation*3 + correction*4 + calibration*2 + metacognition*2)
	negativeSignals := float64(overconfidence * 5)

	words := len(strings.Fields(text))
	if words == 0 {
		return 0, detail
	}

	normalizer := math.Log2(float64(words) + 1)
	if normalizer < 1 {
		normalizer = 1
	}

	rawScore := (positiveSignals - negativeSignals) / normalizer * 10
	score := 50 + rawScore
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
	for _, e := range f.Evidence {
		if e.Marker != "" {
			return e.Marker
		}
	}
	return f.Summary
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

	var models []ModelAwareness
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail AwarenessDetail
		highAware := 0
		lowAware := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			text := evidenceText(e)
			score, detail := scoreResponse(text)
			totalScore += score
			totalDetail.UncertaintyCount += detail.UncertaintyCount
			totalDetail.LimitationCount += detail.LimitationCount
			totalDetail.CorrectionCount += detail.CorrectionCount
			totalDetail.ConfidenceCalibration += detail.ConfidenceCalibration
			totalDetail.OverconfidenceCount += detail.OverconfidenceCount
			totalDetail.MetaCognitionCount += detail.MetaCognitionCount
			if score >= 75 {
				highAware++
			} else if score <= 35 {
				lowAware++
			}
		}

		avg := totalScore / float64(len(entries))
		models = append(models, ModelAwareness{
			Model:          mn,
			AwarenessScore: math.Round(avg*10) / 10,
			Grade:          scoreToGrade(avg),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			HighAwareness:  highAware,
			LowAwareness:   lowAware,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].AwarenessScore > models[j].AwarenessScore
	})

	report := Report{Models: models}
	if len(models) > 0 {
		report.MostAware = models[0].Model
		report.LeastAware = models[len(models)-1].Model
		var sum float64
		for _, m := range models {
			sum += m.AwarenessScore
		}
		report.MeanScore = math.Round(sum/float64(len(models))*10) / 10
	}

	return report
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Self-Awareness Scoring Report")
	fmt.Fprintln(&b, strings.Repeat("=", 50))
	if len(r.Models) == 0 {
		fmt.Fprintln(&b, "  No findings to analyze.")
		return b.String()
	}
	fmt.Fprintf(&b, "  Mean score: %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most aware:  %s\n", r.MostAware)
	fmt.Fprintf(&b, "  Least aware: %s\n\n", r.LeastAware)
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %s  score=%.1f  grade=%s  samples=%d  labs=%d\n",
			m.Model, m.AwarenessScore, m.Grade, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    uncertainty=%d limitation=%d correction=%d calibration=%d overconfidence=%d metacognition=%d\n",
			m.Detail.UncertaintyCount, m.Detail.LimitationCount,
			m.Detail.CorrectionCount, m.Detail.ConfidenceCalibration,
			m.Detail.OverconfidenceCount, m.Detail.MetaCognitionCount)
		fmt.Fprintf(&b, "    high_awareness=%d  low_awareness=%d\n", m.HighAwareness, m.LowAwareness)
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
