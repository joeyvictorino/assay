package instfollow

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ViolationType string

const (
	ViolationFormatMismatch  ViolationType = "format_mismatch"
	ViolationIncomplete      ViolationType = "incomplete"
	ViolationOffTopic        ViolationType = "off_topic"
	ViolationExtraContent    ViolationType = "extra_content"
	ViolationLengthViolation ViolationType = "length_violation"
	ViolationEmpty           ViolationType = "empty_response"
)

type Violation struct {
	Type        ViolationType `json:"type"`
	Description string        `json:"description"`
	Score       float64       `json:"score"`
}

type ResponseScore struct {
	FindingID  string      `json:"finding_id"`
	Model      string      `json:"model"`
	PromptHash string      `json:"prompt_hash"`
	Score      float64     `json:"score"`
	Violations []Violation `json:"violations,omitempty"`
}

type ModelSummary struct {
	Model          string  `json:"model"`
	Total          int     `json:"total"`
	MeanScore      float64 `json:"mean_score"`
	MinScore       float64 `json:"min_score"`
	MaxScore       float64 `json:"max_score"`
	ViolationCount int     `json:"violation_count"`
}

type FailureBreakdown struct {
	Type  ViolationType `json:"type"`
	Count int           `json:"count"`
	Rate  float64       `json:"rate"`
}

type Report struct {
	TotalResponses  int                `json:"total_responses"`
	OverallMean     float64            `json:"overall_mean"`
	ByModel         []ModelSummary     `json:"by_model"`
	FailurePatterns []FailureBreakdown `json:"failure_patterns"`
	Scores          []ResponseScore    `json:"scores"`
}

var formatKeywords = map[string]string{
	"json":     "json",
	"markdown": "markdown",
	"list":     "list",
	"bullet":   "list",
	"numbered": "list",
	"table":    "table",
	"csv":      "csv",
	"xml":      "xml",
	"yaml":     "yaml",
	"code":     "code",
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	var scores []ResponseScore
	modelScores := map[string][]float64{}
	modelViolations := map[string]int{}
	violationCounts := map[ViolationType]int{}

	for _, f := range findings {
		if f.State == model.StateDeclined {
			continue
		}

		rs := scoreResponse(f)
		scores = append(scores, rs)
		modelScores[rs.Model] = append(modelScores[rs.Model], rs.Score)
		modelViolations[rs.Model] += len(rs.Violations)
		for _, v := range rs.Violations {
			violationCounts[v.Type]++
		}
	}

	total := len(scores)
	if total == 0 {
		return Report{}
	}

	var overallSum float64
	for _, s := range scores {
		overallSum += s.Score
	}

	var byModel []ModelSummary
	for m, ss := range modelScores {
		ms := computeModelSummary(m, ss, modelViolations[m])
		byModel = append(byModel, ms)
	}
	sort.Slice(byModel, func(i, j int) bool {
		return byModel[i].MeanScore < byModel[j].MeanScore
	})

	var failures []FailureBreakdown
	for vt, count := range violationCounts {
		failures = append(failures, FailureBreakdown{
			Type:  vt,
			Count: count,
			Rate:  float64(count) / float64(total),
		})
	}
	sort.Slice(failures, func(i, j int) bool {
		return failures[i].Count > failures[j].Count
	})

	return Report{
		TotalResponses:  total,
		OverallMean:     round4(overallSum / float64(total)),
		ByModel:         byModel,
		FailurePatterns: failures,
		Scores:          scores,
	}
}

func scoreResponse(f model.Finding) ResponseScore {
	m := f.Model.Model
	if m == "" {
		m = "unknown"
	}
	summary := f.Summary
	prompt := promptHint(f)

	var violations []Violation
	score := 1.0

	if strings.TrimSpace(summary) == "" {
		violations = append(violations, Violation{
			Type:        ViolationEmpty,
			Description: "response is empty",
			Score:       0.0,
		})
		return ResponseScore{
			FindingID:  f.ID,
			Model:      m,
			PromptHash: f.PromptHash,
			Score:      0.0,
			Violations: violations,
		}
	}

	requestedFmt := detectRequestedFormat(prompt)
	if requestedFmt != "" {
		fmtScore := checkFormatCompliance(summary, requestedFmt)
		if fmtScore < 1.0 {
			violations = append(violations, Violation{
				Type:        ViolationFormatMismatch,
				Description: fmt.Sprintf("expected %s format", requestedFmt),
				Score:       fmtScore,
			})
			score = math.Min(score, fmtScore)
		}
	}

	if len(summary) < 10 && !isShortExpected(prompt) {
		violations = append(violations, Violation{
			Type:        ViolationIncomplete,
			Description: "response appears incomplete",
			Score:       0.3,
		})
		score = math.Min(score, 0.3)
	}

	if containsOffTopic(summary, prompt) {
		violations = append(violations, Violation{
			Type:        ViolationOffTopic,
			Description: "response contains off-topic content",
			Score:       0.5,
		})
		score = math.Min(score, 0.5)
	}

	if containsExtraContent(summary) {
		violations = append(violations, Violation{
			Type:        ViolationExtraContent,
			Description: "response includes unsolicited extra content",
			Score:       0.7,
		})
		score = math.Min(score, 0.7)
	}

	return ResponseScore{
		FindingID:  f.ID,
		Model:      m,
		PromptHash: f.PromptHash,
		Score:      round4(score),
		Violations: violations,
	}
}

func promptHint(f model.Finding) string {
	if f.PromptHash != "" {
		return f.PromptHash
	}
	return f.Summary
}

func detectRequestedFormat(prompt string) string {
	lower := strings.ToLower(prompt)
	for kw, fmt := range formatKeywords {
		if strings.Contains(lower, kw) {
			return fmt
		}
	}
	return ""
}

func checkFormatCompliance(response, format string) float64 {
	switch format {
	case "json":
		trimmed := strings.TrimSpace(response)
		if json.Valid([]byte(trimmed)) {
			return 1.0
		}
		if strings.Contains(trimmed, "{") && strings.Contains(trimmed, "}") {
			return 0.5
		}
		return 0.2
	case "list":
		lines := strings.Split(strings.TrimSpace(response), "\n")
		listLines := 0
		for _, l := range lines {
			t := strings.TrimSpace(l)
			if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") ||
				strings.HasPrefix(t, "1.") || strings.HasPrefix(t, "1)") ||
				(len(t) > 2 && t[0] >= '0' && t[0] <= '9' && (t[1] == '.' || t[1] == ')')) {
				listLines++
			}
		}
		if len(lines) > 0 {
			return math.Min(1.0, float64(listLines)/float64(len(lines))*1.5)
		}
		return 0.0
	case "markdown":
		hasHeaders := strings.Contains(response, "#")
		hasFormatting := strings.Contains(response, "**") || strings.Contains(response, "__") || strings.Contains(response, "```")
		if hasHeaders && hasFormatting {
			return 1.0
		}
		if hasHeaders || hasFormatting {
			return 0.7
		}
		return 0.3
	case "table":
		if strings.Contains(response, "|") && strings.Contains(response, "---") {
			return 1.0
		}
		if strings.Contains(response, "\t") {
			return 0.5
		}
		return 0.2
	case "csv":
		lines := strings.Split(strings.TrimSpace(response), "\n")
		if len(lines) > 1 && strings.Contains(lines[0], ",") {
			return 1.0
		}
		return 0.3
	case "code":
		if strings.Contains(response, "```") || strings.Contains(response, "    ") {
			return 1.0
		}
		return 0.4
	case "xml":
		if strings.Contains(response, "<") && strings.Contains(response, ">") && strings.Contains(response, "</") {
			return 1.0
		}
		return 0.2
	case "yaml":
		lines := strings.Split(strings.TrimSpace(response), "\n")
		yamlLines := 0
		for _, l := range lines {
			if strings.Contains(l, ": ") || strings.HasPrefix(strings.TrimSpace(l), "- ") {
				yamlLines++
			}
		}
		if len(lines) > 0 && float64(yamlLines)/float64(len(lines)) > 0.5 {
			return 1.0
		}
		return 0.4
	}
	return 1.0
}

func isShortExpected(prompt string) bool {
	lower := strings.ToLower(prompt)
	return strings.Contains(lower, "yes or no") ||
		strings.Contains(lower, "true or false") ||
		strings.Contains(lower, "one word")
}

func containsOffTopic(response, prompt string) bool {
	disclaimers := []string{
		"as an ai language model",
		"i should note that",
		"it's important to understand that",
		"before i answer",
		"let me preface this",
	}
	lower := strings.ToLower(response)
	for _, d := range disclaimers {
		if strings.Contains(lower, d) {
			return true
		}
	}
	return false
}

func containsExtraContent(response string) bool {
	extras := []string{
		"hope this helps",
		"let me know if you",
		"feel free to ask",
		"is there anything else",
		"happy to help further",
	}
	lower := strings.ToLower(response)
	for _, e := range extras {
		if strings.Contains(lower, e) {
			return true
		}
	}
	return false
}

func computeModelSummary(m string, scores []float64, violations int) ModelSummary {
	if len(scores) == 0 {
		return ModelSummary{Model: m}
	}
	var sum float64
	minS := 1.0
	maxS := 0.0
	for _, s := range scores {
		sum += s
		if s < minS {
			minS = s
		}
		if s > maxS {
			maxS = s
		}
	}
	return ModelSummary{
		Model:          m,
		Total:          len(scores),
		MeanScore:      round4(sum / float64(len(scores))),
		MinScore:       round4(minS),
		MaxScore:       round4(maxS),
		ViolationCount: violations,
	}
}

func round4(f float64) float64 {
	return math.Round(f*10000) / 10000
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Instruction Following Analysis\n")
	fmt.Fprintf(&b, "%s\n\n", strings.Repeat("=", 62))
	fmt.Fprintf(&b, "  Total responses:   %d\n", r.TotalResponses)
	fmt.Fprintf(&b, "  Overall mean:      %.4f\n\n", r.OverallMean)

	if len(r.ByModel) > 0 {
		fmt.Fprintf(&b, "  By Model:\n")
		fmt.Fprintf(&b, "  %s\n", strings.Repeat("-", 60))
		for _, m := range r.ByModel {
			fmt.Fprintf(&b, "    %-20s mean=%.4f  min=%.4f  max=%.4f  violations=%d\n",
				m.Model, m.MeanScore, m.MinScore, m.MaxScore, m.ViolationCount)
		}
		b.WriteString("\n")
	}

	if len(r.FailurePatterns) > 0 {
		fmt.Fprintf(&b, "  Failure Patterns:\n")
		fmt.Fprintf(&b, "  %s\n", strings.Repeat("-", 60))
		for _, fp := range r.FailurePatterns {
			fmt.Fprintf(&b, "    %-25s count=%d  rate=%.2f%%\n",
				fp.Type, fp.Count, fp.Rate*100)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
