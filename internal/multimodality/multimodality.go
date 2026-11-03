package multimodality

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type MultimodalityDetail struct {
	FormatAwareness   float64 `json:"format_awareness"`
	CrossReference    float64 `json:"cross_reference"`
	VisualDescription float64 `json:"visual_description"`
	DataInterpretation float64 `json:"data_interpretation"`
	MediaIntegration  float64 `json:"media_integration"`
	MultimodalCount   int     `json:"multimodal_count"`
	FormatConfusedCount int   `json:"format_confused_count"`
	AmbiguousCount    int     `json:"ambiguous_count"`
}

type ModelMultimodality struct {
	Model              string              `json:"model"`
	MultimodalityScore float64             `json:"multimodality_score"`
	Grade              string              `json:"grade"`
	Detail             MultimodalityDetail `json:"detail"`
	SampleCount        int                 `json:"sample_count"`
	LabCount           int                 `json:"lab_count"`
	Concerns           int                 `json:"concerns"`
}

type Report struct {
	Models     []ModelMultimodality `json:"models"`
	BestModel  string               `json:"best_model"`
	WorstModel string               `json:"worst_model"`
	MeanScore  float64              `json:"mean_score"`
}

var formatPhrases = []string{
	"the image shows", "in the diagram",
	"the table indicates", "the chart displays",
	"as shown in the figure", "the graph illustrates",
	"the screenshot captures", "the code snippet",
	"the json structure", "the csv data",
	"the pdf document", "the spreadsheet contains",
	"the markdown formatted", "the html page",
}

var crossReferencePhrases = []string{
	"referring to the image above", "as the data shows",
	"cross-referencing with", "comparing the visual with",
	"the text alongside the figure", "correlating the chart data",
	"as mentioned in the table", "linking the diagram to",
	"the caption describes", "corresponding to the visual",
	"matching the pattern in", "as depicted alongside",
}

var visualPhrases = []string{
	"the colors indicate", "the layout shows",
	"visually arranged", "the spatial relationship",
	"positioned to the left", "the visual hierarchy",
	"the arrows point to", "highlighted in",
	"the shading represents", "bordered by",
	"overlapping with", "the visual pattern",
	"prominently displayed", "the focal point",
}

var dataPhrases = []string{
	"the values range from", "the trend shows",
	"the distribution indicates", "the median value",
	"the outliers suggest", "the correlation between",
	"statistically significant", "the mean of",
	"the data points cluster", "the axis represents",
	"the scale ranges", "normalized to",
	"the percentage breakdown", "aggregated across",
}

var mediaPhrases = []string{
	"combining text and image", "the audio indicates",
	"the video segment shows", "integrating multiple formats",
	"the multimedia content", "across different media types",
	"the visual and textual", "multi-format analysis",
	"the embedded content", "cross-media comparison",
	"format-specific handling", "rendering the content",
}

var formatConfusedPhrases = []string{
	"i cannot see the image", "i don't have access to",
	"unable to view", "i can't process the file",
	"the format is not supported", "i cannot read this type",
	"this appears to be binary", "i lack the ability to",
	"visual content is not available", "i cannot interpret the media",
	"text-only limitation", "beyond my capabilities",
}

var ambiguousPhrases = []string{
	"it might be an image", "possibly a chart",
	"appears to contain data", "seems like a table",
	"could be a diagram", "looks like it might be",
	"not sure of the format", "the content type is unclear",
	"may or may not include", "uncertain about the media",
}

func countPhrases(text string, phrases []string) int {
	lower := strings.ToLower(text)
	count := 0
	for _, p := range phrases {
		count += strings.Count(lower, p)
	}
	return count
}

func scoreResponse(response, evidence string) (float64, MultimodalityDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))

	format := countPhrases(combined, formatPhrases)
	crossRef := countPhrases(combined, crossReferencePhrases)
	visual := countPhrases(combined, visualPhrases)
	data := countPhrases(combined, dataPhrases)
	media := countPhrases(combined, mediaPhrases)
	confused := countPhrases(combined, formatConfusedPhrases)
	ambiguous := countPhrases(combined, ambiguousPhrases)

	multimodal := format + crossRef + visual + data + media

	formatScore := 0.2
	if words > 0 {
		formatScore = math.Min(1.0, float64(format)*2.0/float64(words)*10.0+0.2)
	}

	crossRefScore := 0.2
	if words > 0 {
		crossRefScore = math.Min(1.0, float64(crossRef)*2.0/float64(words)*10.0+0.2)
	}

	visualScore := 0.3
	if visual > 0 || data > 0 {
		visualScore = math.Min(1.0, float64(visual+data)*0.15+0.3)
	}

	dataScore := 0.2
	if words > 0 {
		dataScore = math.Min(1.0, float64(data)*2.0/float64(words)*10.0+0.2)
	}

	mediaScore := 0.2
	if confused == 0 && ambiguous == 0 {
		mediaScore = 0.8
	} else if confused+ambiguous < 3 {
		mediaScore = 0.5
	}

	detail := MultimodalityDetail{
		FormatAwareness:     math.Round(formatScore*100) / 100,
		CrossReference:      math.Round(crossRefScore*100) / 100,
		VisualDescription:   math.Round(visualScore*100) / 100,
		DataInterpretation:  math.Round(dataScore*100) / 100,
		MediaIntegration:    math.Round(mediaScore*100) / 100,
		MultimodalCount:     multimodal,
		FormatConfusedCount: confused,
		AmbiguousCount:      ambiguous,
	}

	score := 30.0
	score += formatScore * 15.0
	score += crossRefScore * 15.0
	score += visualScore * 10.0
	score += dataScore * 10.0
	score += mediaScore * 10.0

	score -= float64(confused) * 4.0
	score -= float64(ambiguous) * 5.0

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

	var models []ModelMultimodality
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail MultimodalityDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.FormatAwareness += detail.FormatAwareness
			totalDetail.CrossReference += detail.CrossReference
			totalDetail.VisualDescription += detail.VisualDescription
			totalDetail.DataInterpretation += detail.DataInterpretation
			totalDetail.MediaIntegration += detail.MediaIntegration
			totalDetail.MultimodalCount += detail.MultimodalCount
			totalDetail.FormatConfusedCount += detail.FormatConfusedCount
			totalDetail.AmbiguousCount += detail.AmbiguousCount

			if detail.FormatConfusedCount > 3 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.FormatAwareness = math.Round(totalDetail.FormatAwareness/n*100) / 100
		totalDetail.CrossReference = math.Round(totalDetail.CrossReference/n*100) / 100
		totalDetail.VisualDescription = math.Round(totalDetail.VisualDescription/n*100) / 100
		totalDetail.DataInterpretation = math.Round(totalDetail.DataInterpretation/n*100) / 100
		totalDetail.MediaIntegration = math.Round(totalDetail.MediaIntegration/n*100) / 100

		models = append(models, ModelMultimodality{
			Model:              mn,
			MultimodalityScore: math.Round(avgScore*10) / 10,
			Grade:              scoreToGrade(avgScore),
			Detail:             totalDetail,
			SampleCount:        len(entries),
			LabCount:           len(labs),
			Concerns:           totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].MultimodalityScore > models[j].MultimodalityScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.MultimodalityScore
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
	b.WriteString("Multimodality Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No multimodality data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.MultimodalityScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "format_awareness", m.Detail.FormatAwareness)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "cross_reference", m.Detail.CrossReference)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "visual_description", m.Detail.VisualDescription)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "data_interpretation", m.Detail.DataInterpretation)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "media_integration", m.Detail.MediaIntegration)
		fmt.Fprintf(&b, "    %-28s %d\n", "multimodal_count", m.Detail.MultimodalCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "format_confused_count", m.Detail.FormatConfusedCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "ambiguous_count", m.Detail.AmbiguousCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
