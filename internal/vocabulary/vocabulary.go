package vocabulary

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type VocabularyDetail struct {
	DiversityScore    float64 `json:"diversity_score"`
	PrecisionScore    float64 `json:"precision_score"`
	AccessibilityScore float64 `json:"accessibility_score"`
	RegisterScore     float64 `json:"register_score"`
	ConcisionScore    float64 `json:"concision_score"`
	DiversityCount    int     `json:"diversity_count"`
	PrecisionCount    int     `json:"precision_count"`
	RedundancyCount   int     `json:"redundancy_count"`
}

type ModelVocabulary struct {
	Model           string           `json:"model"`
	VocabularyScore float64          `json:"vocabulary_score"`
	Grade           string           `json:"grade"`
	Detail          VocabularyDetail `json:"detail"`
	SampleCount     int              `json:"sample_count"`
	LabCount        int              `json:"lab_count"`
	Concerns        int              `json:"concerns"`
}

type Report struct {
	Models     []ModelVocabulary `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var diversityPhrases = []string{
	"diverse vocabulary", "varied terminology", "wide range of terms",
	"lexical variety", "synonym usage", "alternative phrasing",
	"word variety", "rich vocabulary", "nuanced language",
	"expressive range", "linguistic diversity", "broad lexicon",
	"multifaceted", "heterogeneous", "assorted",
}

var precisionPhrases = []string{
	"precise terminology", "technical accuracy", "domain-specific",
	"exact term", "proper nomenclature", "correct usage",
	"specific language", "accurate wording", "technically precise",
	"well-defined", "unambiguous term", "designated term",
	"established terminology", "authoritative language", "specialized",
}

var accessibilityPhrases = []string{
	"plain language", "accessible", "clear wording",
	"simple terms", "understandable", "reader-friendly",
	"jargon-free", "straightforward", "easy to follow",
	"approachable", "beginner-friendly", "intuitive",
	"layperson", "common parlance", "everyday language",
}

var registerPhrases = []string{
	"appropriate register", "formal tone", "professional language",
	"suitable style", "context-appropriate", "audience-aware",
	"tone match", "level-appropriate", "fitting register",
	"properly calibrated", "audience-suited", "contextually tuned",
	"register-matched", "style-consistent", "appropriately formal",
}

var redundancyPhrases = []string{
	"redundant", "repetitive", "verbose",
	"unnecessarily wordy", "tautological", "pleonastic",
	"superfluous", "excessively detailed", "overlong",
	"circumlocution", "padding", "filler words",
	"needlessly complex", "bloated", "overwrought",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, VocabularyDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	diversity := countPhrases(combined, diversityPhrases)
	precision := countPhrases(combined, precisionPhrases)
	accessibility := countPhrases(combined, accessibilityPhrases)
	register := countPhrases(combined, registerPhrases)
	redundancy := countPhrases(combined, redundancyPhrases)

	diversityScore := 0.3
	if words > 0 {
		diversityScore = math.Min(1.0, float64(diversity)*2.0/float64(words)*10.0+0.3)
	}

	precisionScore := 0.3
	if precision > 0 {
		precisionScore = math.Min(1.0, float64(precision)*0.12+0.3)
	}

	accessibilityScore := 0.3
	if accessibility > 0 {
		accessibilityScore = math.Min(1.0, float64(accessibility)*0.12+0.3)
	}

	registerScore := 0.2
	if register > 0 {
		registerScore = math.Min(1.0, float64(register)*0.15+0.2)
	}

	concisionScore := 0.5
	if redundancy > 0 {
		concisionScore = math.Max(0, concisionScore-float64(redundancy)*0.1)
	}
	if words > 0 && words < 200 {
		concisionScore = math.Min(1.0, concisionScore+0.2)
	}

	detail := VocabularyDetail{
		DiversityScore:     math.Round(diversityScore*100) / 100,
		PrecisionScore:     math.Round(precisionScore*100) / 100,
		AccessibilityScore: math.Round(accessibilityScore*100) / 100,
		RegisterScore:      math.Round(registerScore*100) / 100,
		ConcisionScore:     math.Round(concisionScore*100) / 100,
		DiversityCount:     diversity,
		PrecisionCount:     precision,
		RedundancyCount:    redundancy,
	}

	score := 25.0
	score += diversityScore * 15.0
	score += precisionScore * 15.0
	score += accessibilityScore * 15.0
	score += registerScore * 10.0
	score += concisionScore * 15.0

	score -= float64(redundancy) * 3.0

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

	var models []ModelVocabulary
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail VocabularyDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.DiversityScore += detail.DiversityScore
			totalDetail.PrecisionScore += detail.PrecisionScore
			totalDetail.AccessibilityScore += detail.AccessibilityScore
			totalDetail.RegisterScore += detail.RegisterScore
			totalDetail.ConcisionScore += detail.ConcisionScore
			totalDetail.DiversityCount += detail.DiversityCount
			totalDetail.PrecisionCount += detail.PrecisionCount
			totalDetail.RedundancyCount += detail.RedundancyCount

			if detail.RedundancyCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.DiversityScore = math.Round(totalDetail.DiversityScore/n*100) / 100
		totalDetail.PrecisionScore = math.Round(totalDetail.PrecisionScore/n*100) / 100
		totalDetail.AccessibilityScore = math.Round(totalDetail.AccessibilityScore/n*100) / 100
		totalDetail.RegisterScore = math.Round(totalDetail.RegisterScore/n*100) / 100
		totalDetail.ConcisionScore = math.Round(totalDetail.ConcisionScore/n*100) / 100

		models = append(models, ModelVocabulary{
			Model:           mn,
			VocabularyScore: math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			Concerns:        totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].VocabularyScore > models[j].VocabularyScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.VocabularyScore
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
	b.WriteString("Vocabulary Sophistication Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No vocabulary data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.VocabularyScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "diversity_score", m.Detail.DiversityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "precision_score", m.Detail.PrecisionScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "accessibility_score", m.Detail.AccessibilityScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "register_score", m.Detail.RegisterScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "concision_score", m.Detail.ConcisionScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "diversity_count", m.Detail.DiversityCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "precision_count", m.Detail.PrecisionCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "redundancy_count", m.Detail.RedundancyCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
