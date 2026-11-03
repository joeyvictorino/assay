package structure

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type StructureDetail struct {
	LogicalOrganization float64 `json:"logical_organization"`
	HeadingUsage        float64 `json:"heading_usage"`
	ListFormatting      float64 `json:"list_formatting"`
	ParagraphCoherence  float64 `json:"paragraph_coherence"`
	TransitionQuality   float64 `json:"transition_quality"`
	HeadingCount        int     `json:"heading_count"`
	ListCount           int     `json:"list_count"`
	TransitionCount     int     `json:"transition_count"`
}

type ModelStructure struct {
	Model          string          `json:"model"`
	StructureScore float64         `json:"structure_score"`
	Grade          string          `json:"grade"`
	Detail         StructureDetail `json:"detail"`
	SampleCount    int             `json:"sample_count"`
	LabCount       int             `json:"lab_count"`
	Concerns       int             `json:"concerns"`
}

type Report struct {
	Models     []ModelStructure `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

var organizationPhrases = []string{
	"first", "second", "third", "fourth",
	"step 1", "step 2", "step 3",
	"to begin", "next", "then",
	"following that", "subsequently",
	"in the first place", "to start with",
	"moving on", "proceeding to",
}

var headingPhrases = []string{
	"overview:", "summary:", "introduction:",
	"background:", "methodology:", "results:",
	"conclusion:", "recommendations:",
	"analysis:", "discussion:",
	"key points:", "main findings:",
	"section", "part 1", "part 2",
}

var listPhrases = []string{
	"1.", "2.", "3.", "4.", "5.",
	"- ", "* ", "a)", "b)", "c)",
	"bullet point", "item:",
	"the following:", "as follows:",
	"listed below", "enumerated",
}

var coherencePhrases = []string{
	"in summary", "to conclude", "therefore",
	"as mentioned", "as discussed",
	"building on", "expanding on",
	"returning to", "as noted above",
	"in other words", "to clarify",
	"that is to say", "more specifically",
}

var transitionPhrases = []string{
	"furthermore", "moreover", "additionally",
	"in addition", "however", "nevertheless",
	"on the contrary", "meanwhile",
	"consequently", "as a result",
	"in contrast", "similarly",
	"likewise", "accordingly",
	"for this reason", "thus",
}

var unstructuredPhrases = []string{
	"by the way", "oh and also",
	"i forgot to mention", "another thing",
	"randomly", "off topic",
	"going back to", "i should have said",
	"let me backtrack", "sorry i digress",
	"actually wait", "one more thing",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, StructureDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	org := countPhrases(combined, organizationPhrases)
	headings := countPhrases(combined, headingPhrases)
	lists := countPhrases(combined, listPhrases)
	coherence := countPhrases(combined, coherencePhrases)
	transitions := countPhrases(combined, transitionPhrases)
	unstructured := countPhrases(combined, unstructuredPhrases)

	orgScore := 0.2
	if words > 0 {
		orgScore = math.Min(1.0, float64(org)*2.0/float64(words)*10.0+0.2)
	}

	headingScore := 0.2
	if headings > 0 {
		headingScore = math.Min(1.0, float64(headings)*0.25+0.2)
	}

	listScore := 0.2
	if lists > 0 {
		listScore = math.Min(1.0, float64(lists)*0.2+0.2)
	}

	coherenceScore := 0.3
	if words > 0 {
		coherenceScore = math.Min(1.0, float64(coherence)*2.0/float64(words)*10.0+0.3)
	}

	transitionScore := 0.2
	if words > 0 {
		transitionScore = math.Min(1.0, float64(transitions)*2.0/float64(words)*10.0+0.2)
	}

	detail := StructureDetail{
		LogicalOrganization: math.Round(orgScore*100) / 100,
		HeadingUsage:        math.Round(headingScore*100) / 100,
		ListFormatting:      math.Round(listScore*100) / 100,
		ParagraphCoherence:  math.Round(coherenceScore*100) / 100,
		TransitionQuality:   math.Round(transitionScore*100) / 100,
		HeadingCount:        headings,
		ListCount:           lists,
		TransitionCount:     transitions,
	}

	score := 30.0
	score += orgScore * 15.0
	score += headingScore * 12.0
	score += listScore * 12.0
	score += coherenceScore * 13.0
	score += transitionScore * 13.0

	score -= float64(unstructured) * 4.0

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

	var models []ModelStructure
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail StructureDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.LogicalOrganization += detail.LogicalOrganization
			totalDetail.HeadingUsage += detail.HeadingUsage
			totalDetail.ListFormatting += detail.ListFormatting
			totalDetail.ParagraphCoherence += detail.ParagraphCoherence
			totalDetail.TransitionQuality += detail.TransitionQuality
			totalDetail.HeadingCount += detail.HeadingCount
			totalDetail.ListCount += detail.ListCount
			totalDetail.TransitionCount += detail.TransitionCount

			if detail.HeadingCount == 0 && detail.ListCount == 0 && detail.TransitionCount == 0 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.LogicalOrganization = math.Round(totalDetail.LogicalOrganization/n*100) / 100
		totalDetail.HeadingUsage = math.Round(totalDetail.HeadingUsage/n*100) / 100
		totalDetail.ListFormatting = math.Round(totalDetail.ListFormatting/n*100) / 100
		totalDetail.ParagraphCoherence = math.Round(totalDetail.ParagraphCoherence/n*100) / 100
		totalDetail.TransitionQuality = math.Round(totalDetail.TransitionQuality/n*100) / 100

		models = append(models, ModelStructure{
			Model:          mn,
			StructureScore: math.Round(avgScore*10) / 10,
			Grade:          scoreToGrade(avgScore),
			Detail:         totalDetail,
			SampleCount:    len(entries),
			LabCount:       len(labs),
			Concerns:       totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].StructureScore > models[j].StructureScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.StructureScore
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
	b.WriteString("Response Structure Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No structure data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.StructureScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "logical_organization", m.Detail.LogicalOrganization)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "heading_usage", m.Detail.HeadingUsage)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "list_formatting", m.Detail.ListFormatting)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "paragraph_coherence", m.Detail.ParagraphCoherence)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "transition_quality", m.Detail.TransitionQuality)
		fmt.Fprintf(&b, "    %-28s %d\n", "heading_count", m.Detail.HeadingCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "list_count", m.Detail.ListCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "transition_count", m.Detail.TransitionCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
