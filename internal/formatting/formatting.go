package formatting

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type FormattingDetail struct {
	MarkdownScore  float64 `json:"markdown_score"`
	CodeBlockScore float64 `json:"code_block_score"`
	ListScore      float64 `json:"list_score"`
	HeadingScore   float64 `json:"heading_score"`
	TableScore     float64 `json:"table_score"`
	WhitespaceScore float64 `json:"whitespace_score"`
	WordCount      int     `json:"word_count"`
	LineCount      int     `json:"line_count"`
}

type ModelFormatting struct {
	Model           string           `json:"model"`
	FormattingScore float64          `json:"formatting_score"`
	Grade           string           `json:"grade"`
	Detail          FormattingDetail `json:"detail"`
	SampleCount     int              `json:"sample_count"`
	LabCount        int              `json:"lab_count"`
	Concerns        int              `json:"concerns"`
}

type Report struct {
	Models     []ModelFormatting `json:"models"`
	BestModel  string            `json:"best_model"`
	WorstModel string            `json:"worst_model"`
	MeanScore  float64           `json:"mean_score"`
}

var markdownIndicators = []string{
	"**", "__", "*", "_", "`", "~~",
	"[", "](", "![",
}

var codeBlockIndicators = []string{
	"```", "~~~",
}

var listIndicators = []string{
	"\n- ", "\n* ", "\n+ ",
	"\n1. ", "\n2. ", "\n3. ",
	"\n4. ", "\n5. ", "\n6. ",
}

var headingIndicators = []string{
	"\n# ", "\n## ", "\n### ",
	"\n#### ", "\n##### ",
}

var tableIndicators = []string{
	"|---", "| ---", "|:---", "| :---",
	"---:|", "--- |",
}

var poorFormattingIndicators = []string{
	"  \n  \n  \n", "\n\n\n\n",
	"........", "--------",
	"========", "********",
}

func countIndicators(text string, indicators []string) int {
	count := 0
	for _, ind := range indicators {
		count += strings.Count(text, ind)
	}
	return count
}

func countLines(text string) int {
	if len(strings.TrimSpace(text)) == 0 {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

func scoreResponse(response, evidence string) (float64, FormattingDetail) {
	combined := response + " " + evidence
	words := len(strings.Fields(combined))
	lines := countLines(combined)

	md := countIndicators(combined, markdownIndicators)
	code := countIndicators(combined, codeBlockIndicators)
	lists := countIndicators(combined, listIndicators)
	headings := countIndicators(combined, headingIndicators)
	tables := countIndicators(combined, tableIndicators)
	poor := countIndicators(combined, poorFormattingIndicators)

	mdScore := math.Min(1.0, float64(md)*0.08+0.2)
	codeScore := math.Min(1.0, float64(code)*0.3+0.1)
	listScore := math.Min(1.0, float64(lists)*0.15+0.1)
	headingScore := math.Min(1.0, float64(headings)*0.25+0.1)
	tableScore := math.Min(1.0, float64(tables)*0.3+0.1)

	wsScore := 0.5
	if lines > 1 && words > 10 {
		avgWordsPerLine := float64(words) / float64(lines)
		if avgWordsPerLine > 5 && avgWordsPerLine < 30 {
			wsScore = 0.8
		}
	}
	if poor > 0 {
		wsScore = math.Max(0, wsScore-float64(poor)*0.2)
	}

	detail := FormattingDetail{
		MarkdownScore:   math.Round(mdScore*100) / 100,
		CodeBlockScore:  math.Round(codeScore*100) / 100,
		ListScore:       math.Round(listScore*100) / 100,
		HeadingScore:    math.Round(headingScore*100) / 100,
		TableScore:      math.Round(tableScore*100) / 100,
		WhitespaceScore: math.Round(wsScore*100) / 100,
		WordCount:       words,
		LineCount:       lines,
	}

	score := 15.0
	score += mdScore * 18.0
	score += codeScore * 15.0
	score += listScore * 18.0
	score += headingScore * 15.0
	score += tableScore * 10.0
	score += wsScore * 9.0

	score -= float64(poor) * 5.0

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

	var models []ModelFormatting
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail FormattingDetail
		totalConcerns := 0

		labs := map[string]bool{}
		for _, e := range entries {
			labs[e.Lab] = true
			evidence := evidenceText(e)
			score, detail := scoreResponse(e.Summary, evidence)
			totalScore += score
			totalDetail.MarkdownScore += detail.MarkdownScore
			totalDetail.CodeBlockScore += detail.CodeBlockScore
			totalDetail.ListScore += detail.ListScore
			totalDetail.HeadingScore += detail.HeadingScore
			totalDetail.TableScore += detail.TableScore
			totalDetail.WhitespaceScore += detail.WhitespaceScore
			totalDetail.WordCount += detail.WordCount
			totalDetail.LineCount += detail.LineCount

			if detail.MarkdownScore < 0.3 && detail.ListScore < 0.2 {
				totalConcerns++
			}
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.MarkdownScore = math.Round(totalDetail.MarkdownScore/n*100) / 100
		totalDetail.CodeBlockScore = math.Round(totalDetail.CodeBlockScore/n*100) / 100
		totalDetail.ListScore = math.Round(totalDetail.ListScore/n*100) / 100
		totalDetail.HeadingScore = math.Round(totalDetail.HeadingScore/n*100) / 100
		totalDetail.TableScore = math.Round(totalDetail.TableScore/n*100) / 100
		totalDetail.WhitespaceScore = math.Round(totalDetail.WhitespaceScore/n*100) / 100

		models = append(models, ModelFormatting{
			Model:           mn,
			FormattingScore: math.Round(avgScore*10) / 10,
			Grade:           scoreToGrade(avgScore),
			Detail:          totalDetail,
			SampleCount:     len(entries),
			LabCount:        len(labs),
			Concerns:        totalConcerns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].FormattingScore > models[j].FormattingScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.FormattingScore
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
	b.WriteString("Response Formatting Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No formatting data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:           %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:          %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.FormattingScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "markdown_score", m.Detail.MarkdownScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "code_block_score", m.Detail.CodeBlockScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "list_score", m.Detail.ListScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "heading_score", m.Detail.HeadingScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "table_score", m.Detail.TableScore)
		fmt.Fprintf(&b, "    %-28s %.2f\n", "whitespace_score", m.Detail.WhitespaceScore)
		fmt.Fprintf(&b, "    %-28s %d\n", "word_count", m.Detail.WordCount)
		fmt.Fprintf(&b, "    %-28s %d\n", "line_count", m.Detail.LineCount)
		fmt.Fprintf(&b, "    Concerns: %d\n", m.Concerns)
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
