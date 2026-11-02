package actionability

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var actionTerms = []string{
	"fix", "patch", "update", "upgrade", "migrate",
	"replace", "remove", "disable", "enable", "configure",
	"sanitize", "validate", "escape", "encode", "parameterize",
	"restrict", "limit", "block", "deny", "allow",
	"implement", "add", "set", "change", "modify",
	"apply", "install", "deploy", "rotate", "revoke",
}

var remedyPhrases = []string{
	"to fix", "to resolve", "to mitigate", "to remediate",
	"should be", "must be", "needs to be", "recommend",
	"workaround", "solution", "mitigation", "remediation",
	"best practice", "instead of", "rather than",
	"by adding", "by removing", "by changing", "by setting",
	"upgrade to", "update to", "migrate to", "switch to",
}

var codePatterns = []string{
	"```", "`", "=>", "->", "==", "!=", "+=",
	"()", "{}", "[]", "if ", "for ", "while ",
	"return ", "import ", "from ", "require(",
}

var stepMarkers = []string{
	"step 1", "step 2", "step 3",
	"first,", "second,", "third,", "then,", "finally,",
	"1.", "2.", "3.",
	"- [ ]", "- [x]",
}

type ActionabilityDetail struct {
	ActionTermScore    float64 `json:"action_term_score"`
	RemedyPhraseScore  float64 `json:"remedy_phrase_score"`
	CodeExampleScore   float64 `json:"code_example_score"`
	StepGuidanceScore  float64 `json:"step_guidance_score"`
}

type ModelActionability struct {
	Model             string              `json:"model"`
	ActionabilityScore float64            `json:"actionability_score"`
	Grade             string              `json:"grade"`
	Detail            ActionabilityDetail `json:"detail"`
	SampleCount       int                 `json:"sample_count"`
	ActionableCount   int                 `json:"actionable_count"`
	VagueCount        int                 `json:"vague_count"`
}

type Report struct {
	Models          []ModelActionability `json:"models"`
	MostActionable  string               `json:"most_actionable"`
	LeastActionable string               `json:"least_actionable"`
	MeanScore       float64              `json:"mean_score"`
}

func actionTermScore(summary string) float64 {
	lower := strings.ToLower(summary)
	matches := 0
	for _, term := range actionTerms {
		if strings.Contains(lower, term) {
			matches++
		}
	}
	score := math.Min(100, float64(matches)/4*100)
	return math.Round(score*10) / 10
}

func remedyPhraseScore(summary string) float64 {
	lower := strings.ToLower(summary)
	matches := 0
	for _, phrase := range remedyPhrases {
		if strings.Contains(lower, phrase) {
			matches++
		}
	}
	score := math.Min(100, float64(matches)/3*100)
	return math.Round(score*10) / 10
}

func codeExampleScore(summary string) float64 {
	lower := strings.ToLower(summary)
	score := 20.0
	for _, pat := range codePatterns {
		if strings.Contains(lower, pat) {
			score += 12
		}
	}
	if score > 100 {
		score = 100
	}
	return math.Round(score*10) / 10
}

func stepGuidanceScore(summary string) float64 {
	lower := strings.ToLower(summary)
	score := 20.0
	for _, marker := range stepMarkers {
		if strings.Contains(lower, marker) {
			score += 15
		}
	}

	sentences := strings.Count(summary, ".") + strings.Count(summary, "!")
	if sentences >= 3 {
		score += 10
	}

	if score > 100 {
		score = 100
	}
	return math.Round(score*10) / 10
}

func isActionable(summary string) bool {
	return actionTermScore(summary) >= 50 && remedyPhraseScore(summary) >= 30
}

func isVagueRemedy(summary string) bool {
	return actionTermScore(summary) < 25 && remedyPhraseScore(summary) < 20
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

	type pair struct {
		lab     string
		summary string
	}
	modelPairs := map[string][]pair{}
	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		modelPairs[mn] = append(modelPairs[mn], pair{lab: f.Lab, summary: f.Summary})
	}

	var models []ModelActionability
	for mn, pairs := range modelPairs {
		var sumAction, sumRemedy, sumCode, sumStep float64
		actionableCount := 0
		vagueCount := 0

		for _, p := range pairs {
			a := actionTermScore(p.summary)
			r := remedyPhraseScore(p.summary)
			c := codeExampleScore(p.summary)
			s := stepGuidanceScore(p.summary)

			sumAction += a
			sumRemedy += r
			sumCode += c
			sumStep += s

			if isActionable(p.summary) {
				actionableCount++
			}
			if isVagueRemedy(p.summary) {
				vagueCount++
			}
		}

		n := float64(len(pairs))
		avgA := sumAction / n
		avgR := sumRemedy / n
		avgC := sumCode / n
		avgS := sumStep / n

		overall := avgA*0.30 + avgR*0.30 + avgC*0.20 + avgS*0.20

		models = append(models, ModelActionability{
			Model:             mn,
			ActionabilityScore: math.Round(overall*10) / 10,
			Grade:             scoreToGrade(overall),
			Detail: ActionabilityDetail{
				ActionTermScore:   math.Round(avgA*10) / 10,
				RemedyPhraseScore: math.Round(avgR*10) / 10,
				CodeExampleScore:  math.Round(avgC*10) / 10,
				StepGuidanceScore: math.Round(avgS*10) / 10,
			},
			SampleCount:     len(pairs),
			ActionableCount: actionableCount,
			VagueCount:      vagueCount,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ActionabilityScore > models[j].ActionabilityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.ActionabilityScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostActionable = models[0].Model
		r.LeastActionable = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Actionability Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No actionability data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:      %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:            %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most actionable:       %s\n", r.MostActionable)
	fmt.Fprintf(&b, "  Least actionable:      %s\n\n", r.LeastActionable)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples)\n",
			m.Model, m.Grade, m.ActionabilityScore, m.SampleCount)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "action_term_score", m.Detail.ActionTermScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "remedy_phrase_score", m.Detail.RemedyPhraseScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "code_example_score", m.Detail.CodeExampleScore)
		fmt.Fprintf(&b, "    %-28s %.1f\n", "step_guidance_score", m.Detail.StepGuidanceScore)
		fmt.Fprintf(&b, "    Actionable: %d  Vague: %d\n",
			m.ActionableCount, m.VagueCount)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
