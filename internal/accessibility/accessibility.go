package accessibility

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type AccessibilityDetail struct {
	PlainLanguage    float64 `json:"plain_language"`
	StructureClarity float64 `json:"structure_clarity"`
	ExampleUse       float64 `json:"example_use"`
	JargonAvoidance  float64 `json:"jargon_avoidance"`
	AudienceAwareness float64 `json:"audience_awareness"`
	AccessibleCount  int     `json:"accessible_count"`
	JargonCount      int     `json:"jargon_count"`
	ComplexCount     int     `json:"complex_count"`
}

type ModelAccessibility struct {
	Model              string              `json:"model"`
	AccessibilityScore float64             `json:"accessibility_score"`
	Grade              string              `json:"grade"`
	Detail             AccessibilityDetail `json:"detail"`
	SampleCount        int                 `json:"sample_count"`
	LabCount           int                 `json:"lab_count"`
	Concerns           int                 `json:"concerns"`
}

type Report struct {
	Models     []ModelAccessibility `json:"models"`
	BestModel  string               `json:"best_model"`
	WorstModel string               `json:"worst_model"`
	MeanScore  float64              `json:"mean_score"`
}

var plainPhrases = []string{
	"in simple terms", "put simply", "in other words",
	"this means", "to clarify", "essentially",
	"the basic idea", "at its core", "simply put",
	"think of it as", "a good analogy", "for example",
	"to illustrate", "in everyday terms", "plainly speaking",
}

var structurePhrases = []string{
	"first", "second", "third",
	"step 1", "step 2", "step 3",
	"to begin", "next", "finally",
	"in summary", "the key takeaway", "to recap",
	"here is an overview", "the main points", "let me break this down",
}

var examplePhrases = []string{
	"for example", "for instance", "such as",
	"consider this scenario", "imagine", "suppose",
	"a real-world example", "to demonstrate", "here is an example",
	"a common case", "in practice", "concretely",
	"to make this concrete", "as an illustration", "a typical use case",
}

var audiencePhrases = []string{
	"if you are new to", "for beginners", "no prior knowledge",
	"even without experience", "anyone can understand",
	"you do not need to be an expert", "accessible to all",
	"regardless of background", "user-friendly",
	"non-technical audience", "easy to follow",
	"straightforward", "approachable", "intuitive",
}

var jargonPhrases = []string{
	"eigenvalue", "heuristic", "ontological",
	"epistemological", "orthogonal", "idempotent",
	"polymorphism", "monadic", "functorial",
	"isomorphic", "homeomorphic", "bijective",
	"stochastic", "deterministic gradient",
}

var complexPhrases = []string{
	"it is trivially obvious", "as is well known",
	"the proof is left", "by induction on",
	"without loss of generality", "it can be shown",
	"the reader will note", "a fortiori",
	"ipso facto", "mutatis mutandis",
	"ceteris paribus", "qed",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, AccessibilityDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	plain := countPhrases(combined, plainPhrases)
	structure := countPhrases(combined, structurePhrases)
	examples := countPhrases(combined, examplePhrases)
	audience := countPhrases(combined, audiencePhrases)
	jargon := countPhrases(combined, jargonPhrases)
	complex := countPhrases(combined, complexPhrases)

	accessible := plain + structure + examples + audience

	plainScore := 0.2
	if words > 0 {
		plainScore = math.Min(1.0, float64(plain)*2.0/float64(words)*10.0+0.2)
	}

	structScore := 0.2
	if words > 0 {
		structScore = math.Min(1.0, float64(structure)*2.0/float64(words)*10.0+0.2)
	}

	exampleScore := 0.3
	if examples > 0 {
		exampleScore = math.Min(1.0, float64(examples)*0.15+0.3)
	}

	audienceScore := 0.2
	if words > 0 {
		audienceScore = math.Min(1.0, float64(audience)*2.0/float64(words)*10.0+0.2)
	}

	jargonScore := 0.8
	if jargon > 0 {
		jargonScore = math.Max(0.1, 0.8-float64(jargon)*0.15)
	}

	detail := AccessibilityDetail{
		PlainLanguage:     math.Round(plainScore*100) / 100,
		StructureClarity:  math.Round(structScore*100) / 100,
		ExampleUse:        math.Round(exampleScore*100) / 100,
		JargonAvoidance:   math.Round(jargonScore*100) / 100,
		AudienceAwareness: math.Round(audienceScore*100) / 100,
		AccessibleCount:   accessible,
		JargonCount:       jargon,
		ComplexCount:      complex,
	}

	score := 30.0
	score += plainScore * 15.0
	score += structScore * 15.0
	score += exampleScore * 10.0
	score += jargonScore * 10.0
	score += audienceScore * 10.0

	score -= float64(jargon) * 4.0
	score -= float64(complex) * 5.0

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

	var models []ModelAccessibility
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail AccessibilityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			ev := evidenceText(e)
			score, detail := scoreResponse(e.Summary, ev)
			totalScore += score
			totalDetail.PlainLanguage += detail.PlainLanguage
			totalDetail.StructureClarity += detail.StructureClarity
			totalDetail.ExampleUse += detail.ExampleUse
			totalDetail.JargonAvoidance += detail.JargonAvoidance
			totalDetail.AudienceAwareness += detail.AudienceAwareness
			totalDetail.AccessibleCount += detail.AccessibleCount
			totalDetail.JargonCount += detail.JargonCount
			totalDetail.ComplexCount += detail.ComplexCount

			if detail.JargonCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.PlainLanguage = math.Round(totalDetail.PlainLanguage/n*100) / 100
		totalDetail.StructureClarity = math.Round(totalDetail.StructureClarity/n*100) / 100
		totalDetail.ExampleUse = math.Round(totalDetail.ExampleUse/n*100) / 100
		totalDetail.JargonAvoidance = math.Round(totalDetail.JargonAvoidance/n*100) / 100
		totalDetail.AudienceAwareness = math.Round(totalDetail.AudienceAwareness/n*100) / 100

		models = append(models, ModelAccessibility{
			Model:              mn,
			AccessibilityScore: math.Round(avgScore*10) / 10,
			Grade:              scoreToGrade(avgScore),
			Detail:             totalDetail,
			SampleCount:        len(entries),
			LabCount:           len(labs),
			Concerns:           totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].AccessibilityScore > models[j].AccessibilityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.AccessibilityScore
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
	b.WriteString("Response Accessibility Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No accessibility data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.AccessibilityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "plain_language", m.Detail.PlainLanguage)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "structure_clarity", m.Detail.StructureClarity)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "example_use", m.Detail.ExampleUse)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "jargon_avoidance", m.Detail.JargonAvoidance)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "audience_awareness", m.Detail.AudienceAwareness)
		fmt.Fprintf(&b, "    %-28s %d\n", "accessible_count", m.Detail.AccessibleCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "jargon_count", m.Detail.JargonCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "complex_count", m.Detail.ComplexCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
