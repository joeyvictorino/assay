package fmtcheck

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type FormatType string

const (
	FmtJSON     FormatType = "json"
	FmtMarkdown FormatType = "markdown"
	FmtList     FormatType = "list"
	FmtCode     FormatType = "code"
	FmtAny      FormatType = "any"
)

type Violation struct {
	Finding    string     `json:"finding_id"`
	Model      string     `json:"model"`
	Format     FormatType `json:"expected_format"`
	Score      float64    `json:"score"`
	Issues     []string   `json:"issues"`
}

type ModelCompliance struct {
	Model         string  `json:"model"`
	Total         int     `json:"total"`
	MeanScore     float64 `json:"mean_score"`
	PerfectCount  int     `json:"perfect_count"`
	WorstScore    float64 `json:"worst_score"`
}

type FormatCompliance struct {
	Format    FormatType `json:"format"`
	Total     int        `json:"total"`
	MeanScore float64    `json:"mean_score"`
}

type Report struct {
	TotalChecked   int                `json:"total_checked"`
	MeanScore      float64            `json:"mean_score"`
	ByModel        []ModelCompliance  `json:"by_model"`
	ByFormat       []FormatCompliance `json:"by_format"`
	Violations     []Violation        `json:"violations"`
	WorstPrompts   []string           `json:"worst_prompts,omitempty"`
}

func DetectExpectedFormat(summary string) FormatType {
	lower := strings.ToLower(summary)
	if strings.Contains(lower, "json") {
		return FmtJSON
	}
	if strings.Contains(lower, "markdown") || strings.Contains(lower, "## ") {
		return FmtMarkdown
	}
	if strings.Contains(lower, "list") || strings.Contains(lower, "enumerate") {
		return FmtList
	}
	if strings.Contains(lower, "code") || strings.Contains(lower, "```") {
		return FmtCode
	}
	return FmtAny
}

func ScoreCompliance(summary string, format FormatType) (float64, []string) {
	if summary == "" {
		return 0.0, []string{"empty response"}
	}

	switch format {
	case FmtJSON:
		return scoreJSON(summary)
	case FmtMarkdown:
		return scoreMarkdown(summary)
	case FmtList:
		return scoreList(summary)
	case FmtCode:
		return scoreCode(summary)
	default:
		return scoreGeneral(summary)
	}
}

func scoreJSON(s string) (float64, []string) {
	var issues []string
	trimmed := strings.TrimSpace(s)

	if json.Valid([]byte(trimmed)) {
		return 1.0, nil
	}

	// Check for partial JSON structures
	score := 0.0
	if strings.Contains(trimmed, "{") && strings.Contains(trimmed, "}") {
		score = 0.4
		issues = append(issues, "contains braces but invalid JSON")
	} else if strings.Contains(trimmed, "[") && strings.Contains(trimmed, "]") {
		score = 0.3
		issues = append(issues, "contains brackets but invalid JSON")
	} else {
		issues = append(issues, "no JSON structure found")
	}

	// Bonus if keys look JSON-like
	if matched, _ := regexp.MatchString(`"[a-zA-Z_]+"\s*:`, trimmed); matched {
		score += 0.2
	}

	if score > 1.0 {
		score = 1.0
	}
	return score, issues
}

func scoreMarkdown(s string) (float64, []string) {
	var issues []string
	score := 0.0
	lines := strings.Split(s, "\n")

	hasHeading := false
	hasList := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			hasHeading = true
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") ||
			(len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && trimmed[1] == '.') {
			hasList = true
		}
	}

	if hasHeading {
		score += 0.5
	} else {
		issues = append(issues, "no markdown headings")
	}
	if hasList {
		score += 0.3
	}
	if len(lines) > 1 {
		score += 0.2
	} else {
		issues = append(issues, "single-line response")
	}

	if score > 1.0 {
		score = 1.0
	}
	return score, issues
}

func scoreList(s string) (float64, []string) {
	var issues []string
	lines := strings.Split(strings.TrimSpace(s), "\n")

	listItems := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") ||
			strings.HasPrefix(trimmed, "• ") {
			listItems++
		} else if len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && (trimmed[1] == '.' || trimmed[1] == ')') {
			listItems++
		}
	}

	if listItems == 0 {
		return 0.2, append(issues, "no list items found")
	}

	ratio := float64(listItems) / float64(len(lines))
	score := 0.3 + ratio*0.7
	if score > 1.0 {
		score = 1.0
	}
	return score, issues
}

func scoreCode(s string) (float64, []string) {
	var issues []string
	score := 0.0

	if strings.Contains(s, "```") {
		blocks := strings.Count(s, "```")
		if blocks >= 2 && blocks%2 == 0 {
			score = 1.0
		} else {
			score = 0.5
			issues = append(issues, "unclosed code block")
		}
	} else {
		score = 0.3
		issues = append(issues, "no code fences found")
	}

	return score, issues
}

func scoreGeneral(s string) (float64, []string) {
	if len(s) < 10 {
		return 0.5, []string{"very short response"}
	}
	return 1.0, nil
}

func Analyze(findings []model.Finding) Report {
	type modelAcc struct {
		total        int
		sumScore     float64
		perfectCount int
		worstScore   float64
	}
	type formatAcc struct {
		total    int
		sumScore float64
	}

	byModel := make(map[string]*modelAcc)
	byFormat := make(map[FormatType]*formatAcc)
	var violations []Violation
	promptScores := make(map[string][]float64)
	totalScore := 0.0

	for _, f := range findings {
		format := DetectExpectedFormat(f.Summary)
		score, issues := ScoreCompliance(f.Summary, format)
		totalScore += score

		m := f.Model.Model
		if byModel[m] == nil {
			byModel[m] = &modelAcc{worstScore: 1.0}
		}
		byModel[m].total++
		byModel[m].sumScore += score
		if score == 1.0 {
			byModel[m].perfectCount++
		}
		if score < byModel[m].worstScore {
			byModel[m].worstScore = score
		}

		if byFormat[format] == nil {
			byFormat[format] = &formatAcc{}
		}
		byFormat[format].total++
		byFormat[format].sumScore += score

		ph := f.PromptHash
		if ph == "" {
			ph = f.ID
		}
		promptScores[ph] = append(promptScores[ph], score)

		if score < 1.0 {
			violations = append(violations, Violation{
				Finding: f.ID,
				Model:   m,
				Format:  format,
				Score:   round2(score),
				Issues:  issues,
			})
		}
	}

	sort.Slice(violations, func(i, j int) bool {
		return violations[i].Score < violations[j].Score
	})

	var modelComps []ModelCompliance
	for m, a := range byModel {
		mean := 0.0
		if a.total > 0 {
			mean = a.sumScore / float64(a.total)
		}
		modelComps = append(modelComps, ModelCompliance{
			Model:        m,
			Total:        a.total,
			MeanScore:    round2(mean),
			PerfectCount: a.perfectCount,
			WorstScore:   round2(a.worstScore),
		})
	}
	sort.Slice(modelComps, func(i, j int) bool {
		if modelComps[i].MeanScore != modelComps[j].MeanScore {
			return modelComps[i].MeanScore < modelComps[j].MeanScore
		}
		return modelComps[i].Model < modelComps[j].Model
	})

	var formatComps []FormatCompliance
	for f, a := range byFormat {
		mean := 0.0
		if a.total > 0 {
			mean = a.sumScore / float64(a.total)
		}
		formatComps = append(formatComps, FormatCompliance{
			Format:    f,
			Total:     a.total,
			MeanScore: round2(mean),
		})
	}
	sort.Slice(formatComps, func(i, j int) bool {
		return formatComps[i].MeanScore < formatComps[j].MeanScore
	})

	// Worst prompts by average score
	type promptAvg struct {
		hash string
		avg  float64
	}
	var avgs []promptAvg
	for ph, scores := range promptScores {
		sum := 0.0
		for _, s := range scores {
			sum += s
		}
		avgs = append(avgs, promptAvg{ph, sum / float64(len(scores))})
	}
	sort.Slice(avgs, func(i, j int) bool { return avgs[i].avg < avgs[j].avg })
	var worstPrompts []string
	for i, a := range avgs {
		if i >= 5 || a.avg >= 1.0 {
			break
		}
		worstPrompts = append(worstPrompts, a.hash)
	}

	meanScore := 0.0
	if len(findings) > 0 {
		meanScore = totalScore / float64(len(findings))
	}

	return Report{
		TotalChecked: len(findings),
		MeanScore:    round2(meanScore),
		ByModel:      modelComps,
		ByFormat:      formatComps,
		Violations:   violations,
		WorstPrompts: worstPrompts,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Output Format Compliance Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	fmt.Fprintf(&b, "  Findings checked:       %d\n", r.TotalChecked)
	fmt.Fprintf(&b, "  Mean compliance score:  %.2f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Violations:             %d\n\n", len(r.Violations))

	if len(r.ByModel) > 0 {
		b.WriteString("  By Model:\n")
		b.WriteString("  " + strings.Repeat("-", 58) + "\n")
		for _, m := range r.ByModel {
			fmt.Fprintf(&b, "    %-25s mean=%.2f  perfect=%d/%d  worst=%.2f\n",
				m.Model, m.MeanScore, m.PerfectCount, m.Total, m.WorstScore)
		}
		b.WriteString("\n")
	}

	if len(r.ByFormat) > 0 {
		b.WriteString("  By Format:\n")
		b.WriteString("  " + strings.Repeat("-", 58) + "\n")
		for _, f := range r.ByFormat {
			fmt.Fprintf(&b, "    %-15s %3d checked  mean=%.2f\n", f.Format, f.Total, f.MeanScore)
		}
		b.WriteString("\n")
	}

	if len(r.WorstPrompts) > 0 {
		b.WriteString("  Worst Prompts:\n")
		b.WriteString("  " + strings.Repeat("-", 58) + "\n")
		for _, p := range r.WorstPrompts {
			fmt.Fprintf(&b, "    %s\n", ellipsis(p, 56))
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b) + "\n"
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func ellipsis(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
