package fmtcomply

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

type FormatType string

const (
	FormatJSON       FormatType = "json"
	FormatXML        FormatType = "xml"
	FormatCSV        FormatType = "csv"
	FormatMarkdown   FormatType = "markdown"
	FormatNumbered   FormatType = "numbered_list"
	FormatBulleted   FormatType = "bulleted_list"
	FormatCodeBlock  FormatType = "code_block"
	FormatKeyValue   FormatType = "key_value"
)

type Sample struct {
	Prompt         string `json:"prompt"`
	Response       string `json:"response"`
	RequestedFormat FormatType `json:"requested_format"`
	Model          string `json:"model"`
}

type FormatCheck struct {
	Valid            bool     `json:"valid"`
	StructuralScore  float64  `json:"structural_score"`
	Violations       []string `json:"violations"`
}

type ModelCompliance struct {
	Model           string  `json:"model"`
	OverallScore    float64 `json:"overall_score"`
	Grade           string  `json:"grade"`
	SampleCount     int     `json:"sample_count"`
	FullyCompliant  int     `json:"fully_compliant"`
	PartiallyCompliant int  `json:"partially_compliant"`
	NonCompliant    int     `json:"non_compliant"`
}

type Report struct {
	Models          []ModelCompliance `json:"models"`
	BestCompliance  string            `json:"best_compliance"`
	WorstCompliance string            `json:"worst_compliance"`
	MeanScore       float64           `json:"mean_score"`
}

func CheckFormat(response string, format FormatType) FormatCheck {
	switch format {
	case FormatJSON:
		return checkJSON(response)
	case FormatXML:
		return checkXML(response)
	case FormatCSV:
		return checkCSV(response)
	case FormatMarkdown:
		return checkMarkdown(response)
	case FormatNumbered:
		return checkNumberedList(response)
	case FormatBulleted:
		return checkBulletedList(response)
	case FormatCodeBlock:
		return checkCodeBlock(response)
	case FormatKeyValue:
		return checkKeyValue(response)
	default:
		return FormatCheck{Valid: false, Violations: []string{"unknown format type"}}
	}
}

func checkJSON(s string) FormatCheck {
	s = extractBlock(s, "json")
	var violations []string
	var js json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &js); err != nil {
		violations = append(violations, "invalid JSON: "+err.Error())
		if strings.Contains(s, "{") || strings.Contains(s, "[") {
			return FormatCheck{Valid: false, StructuralScore: 30, Violations: violations}
		}
		return FormatCheck{Valid: false, StructuralScore: 0, Violations: violations}
	}
	return FormatCheck{Valid: true, StructuralScore: 100}
}

func checkXML(s string) FormatCheck {
	s = extractBlock(s, "xml")
	trimmed := strings.TrimSpace(s)
	var violations []string
	if !strings.Contains(trimmed, "<") || !strings.Contains(trimmed, ">") {
		violations = append(violations, "no XML tags found")
		return FormatCheck{Valid: false, StructuralScore: 0, Violations: violations}
	}
	openCount := strings.Count(trimmed, "<") - strings.Count(trimmed, "</") - strings.Count(trimmed, "/>")
	closeCount := strings.Count(trimmed, "</")
	if openCount != closeCount {
		violations = append(violations, fmt.Sprintf("mismatched tags: %d open vs %d close", openCount, closeCount))
		score := math.Max(0, 100-float64(abs(openCount-closeCount))*20)
		return FormatCheck{Valid: false, StructuralScore: score, Violations: violations}
	}
	return FormatCheck{Valid: true, StructuralScore: 100}
}

func checkCSV(s string) FormatCheck {
	s = extractBlock(s, "csv")
	lines := nonEmptyLines(s)
	var violations []string
	if len(lines) == 0 {
		violations = append(violations, "no CSV lines found")
		return FormatCheck{Valid: false, StructuralScore: 0, Violations: violations}
	}
	headerCols := strings.Count(lines[0], ",") + 1
	inconsistent := 0
	for i, line := range lines[1:] {
		cols := strings.Count(line, ",") + 1
		if cols != headerCols {
			inconsistent++
			if inconsistent <= 3 {
				violations = append(violations, fmt.Sprintf("row %d has %d columns, header has %d", i+2, cols, headerCols))
			}
		}
	}
	if inconsistent > 0 {
		score := math.Max(0, 100-float64(inconsistent)/float64(len(lines))*100)
		return FormatCheck{Valid: false, StructuralScore: score, Violations: violations}
	}
	return FormatCheck{Valid: true, StructuralScore: 100}
}

func checkMarkdown(s string) FormatCheck {
	var violations []string
	hasHeading := false
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			hasHeading = true
			break
		}
	}
	if !hasHeading {
		violations = append(violations, "no markdown headings found")
	}
	score := 100.0
	if !hasHeading {
		score -= 40
	}
	if !strings.Contains(s, "**") && !strings.Contains(s, "*") && !strings.Contains(s, "`") {
		score -= 20
		violations = append(violations, "no markdown formatting (bold, italic, code) found")
	}
	return FormatCheck{Valid: len(violations) == 0, StructuralScore: math.Max(0, score), Violations: violations}
}

func checkNumberedList(s string) FormatCheck {
	lines := nonEmptyLines(s)
	var violations []string
	numbered := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) > 1 && trimmed[0] >= '1' && trimmed[0] <= '9' && strings.Contains(trimmed[:min(5, len(trimmed))], ".") {
			numbered++
		}
	}
	if numbered == 0 {
		violations = append(violations, "no numbered list items found")
		return FormatCheck{Valid: false, StructuralScore: 0, Violations: violations}
	}
	ratio := float64(numbered) / float64(len(lines))
	if ratio < 0.5 {
		violations = append(violations, fmt.Sprintf("only %d/%d lines are numbered", numbered, len(lines)))
	}
	score := math.Min(100, ratio*120)
	return FormatCheck{Valid: len(violations) == 0, StructuralScore: score, Violations: violations}
}

func checkBulletedList(s string) FormatCheck {
	lines := nonEmptyLines(s)
	var violations []string
	bulleted := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "• ") {
			bulleted++
		}
	}
	if bulleted == 0 {
		violations = append(violations, "no bulleted list items found")
		return FormatCheck{Valid: false, StructuralScore: 0, Violations: violations}
	}
	ratio := float64(bulleted) / float64(len(lines))
	if ratio < 0.5 {
		violations = append(violations, fmt.Sprintf("only %d/%d lines are bulleted", bulleted, len(lines)))
	}
	score := math.Min(100, ratio*120)
	return FormatCheck{Valid: len(violations) == 0, StructuralScore: score, Violations: violations}
}

func checkCodeBlock(s string) FormatCheck {
	var violations []string
	if !strings.Contains(s, "```") {
		violations = append(violations, "no code block fences found")
		return FormatCheck{Valid: false, StructuralScore: 0, Violations: violations}
	}
	opens := 0
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			opens++
		}
	}
	if opens%2 != 0 {
		violations = append(violations, "unclosed code block fence")
		return FormatCheck{Valid: false, StructuralScore: 60, Violations: violations}
	}
	return FormatCheck{Valid: true, StructuralScore: 100}
}

func checkKeyValue(s string) FormatCheck {
	lines := nonEmptyLines(s)
	var violations []string
	kvCount := 0
	for _, line := range lines {
		if strings.Contains(line, ":") || strings.Contains(line, "=") {
			kvCount++
		}
	}
	if kvCount == 0 {
		violations = append(violations, "no key-value pairs found")
		return FormatCheck{Valid: false, StructuralScore: 0, Violations: violations}
	}
	ratio := float64(kvCount) / float64(len(lines))
	score := math.Min(100, ratio*110)
	return FormatCheck{Valid: ratio >= 0.5, StructuralScore: score, Violations: violations}
}

func Analyze(samples []Sample) Report {
	byModel := map[string][]Sample{}
	for _, s := range samples {
		byModel[s.Model] = append(byModel[s.Model], s)
	}

	var models []ModelCompliance
	for modelName, modelSamples := range byModel {
		mc := ModelCompliance{Model: modelName, SampleCount: len(modelSamples)}
		var totalScore float64
		for _, s := range modelSamples {
			check := CheckFormat(s.Response, s.RequestedFormat)
			totalScore += check.StructuralScore
			if check.Valid && check.StructuralScore >= 90 {
				mc.FullyCompliant++
			} else if check.StructuralScore >= 50 {
				mc.PartiallyCompliant++
			} else {
				mc.NonCompliant++
			}
		}
		mc.OverallScore = totalScore / float64(len(modelSamples))
		mc.Grade = scoreToGrade(mc.OverallScore)
		models = append(models, mc)
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].OverallScore > models[j].OverallScore
	})

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestCompliance = models[0].Model
		r.WorstCompliance = models[len(models)-1].Model
		var sum float64
		for _, m := range models {
			sum += m.OverallScore
		}
		r.MeanScore = sum / float64(len(models))
	}
	return r
}

func scoreToGrade(score float64) string {
	switch {
	case score >= 95:
		return "A+"
	case score >= 90:
		return "A"
	case score >= 85:
		return "A-"
	case score >= 80:
		return "B+"
	case score >= 75:
		return "B"
	case score >= 70:
		return "B-"
	case score >= 65:
		return "C+"
	case score >= 60:
		return "C"
	case score >= 50:
		return "D"
	default:
		return "F"
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Format Compliance Analysis (%d models, mean score %.1f)\n\n", len(r.Models), r.MeanScore)
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %-20s  score: %5.1f  grade: %-2s  (%d samples: %d full, %d partial, %d non-compliant)\n",
			m.Model, m.OverallScore, m.Grade, m.SampleCount, m.FullyCompliant, m.PartiallyCompliant, m.NonCompliant)
	}
	if r.BestCompliance != "" {
		fmt.Fprintf(&b, "\n  Best:  %s\n  Worst: %s\n", r.BestCompliance, r.WorstCompliance)
	}
	return b.String()
}

func extractBlock(s string, lang string) string {
	marker := "```" + lang
	idx := strings.Index(s, marker)
	if idx == -1 {
		marker = "```"
		idx = strings.Index(s, marker)
	}
	if idx == -1 {
		return s
	}
	rest := s[idx+len(marker):]
	if nl := strings.Index(rest, "\n"); nl >= 0 {
		rest = rest[nl+1:]
	}
	end := strings.Index(rest, "```")
	if end == -1 {
		return rest
	}
	return rest[:end]
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
