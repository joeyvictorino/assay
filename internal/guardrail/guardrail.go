package guardrail

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type CheckerResult struct {
	Checker string  `json:"checker"`
	Passed  bool    `json:"passed"`
	Score   float64 `json:"score"`
	Details string  `json:"details"`
}

type GuardrailCategory struct {
	Name       string          `json:"name"`
	PassRate   float64         `json:"pass_rate"`
	TotalTests int             `json:"total_tests"`
	Passed     int             `json:"passed"`
	Failed     int             `json:"failed"`
	Checkers   []CheckerResult `json:"checkers"`
}

type GuardrailReport struct {
	Categories    []GuardrailCategory `json:"categories"`
	OverallPass   float64             `json:"overall_pass_rate"`
	WeakestArea   string              `json:"weakest_area"`
	StrongestArea string              `json:"strongest_area"`
}

var categoryKeywords = map[string][]string{
	"prompt_injection": {"injection", "prompt", "jailbreak"},
	"data_leakage":    {"leak", "disclosure", "sensitive", "exfil", "info-disclosure"},
	"harmful_content": {"harmful", "toxic", "abuse", "unsafe"},
	"hallucination":   {"hallucin", "fabricat", "confabul", "ground"},
	"bias":            {"bias", "fairness", "stereotyp", "discriminat"},
	"tool_misuse":     {"tool", "function", "capability", "delegate"},
	"token_limit":     {"token", "limit", "budget", "length", "truncat"},
	"output_format":   {"format", "schema", "structur", "parse", "json", "output"},
}

var categoryOrder = []string{
	"prompt_injection",
	"data_leakage",
	"harmful_content",
	"hallucination",
	"bias",
	"tool_misuse",
	"token_limit",
	"output_format",
}

func ClassifyChecker(lab string) string {
	lower := strings.ToLower(lab)
	for _, cat := range categoryOrder {
		for _, kw := range categoryKeywords[cat] {
			if strings.Contains(lower, kw) {
				return cat
			}
		}
	}
	classMap := map[model.Class]string{
		model.ClassSQLi:           "prompt_injection",
		model.ClassXSSReflected:   "prompt_injection",
		model.ClassXSSStored:      "prompt_injection",
		model.ClassSSRF:           "prompt_injection",
		model.ClassInfoDisclosure: "data_leakage",
		model.ClassSensitiveFile:  "data_leakage",
		model.ClassVerboseError:   "data_leakage",
		model.ClassAuthMissing:    "tool_misuse",
		model.ClassAuthBypass:     "tool_misuse",
		model.ClassDefaultCreds:   "tool_misuse",
		model.ClassIDOR:           "data_leakage",
		model.ClassPathTraversal:  "data_leakage",
		model.ClassOpenRedirect:   "prompt_injection",
	}
	for cls, cat := range classMap {
		if strings.Contains(lower, string(cls)) {
			return cat
		}
	}
	return ""
}

func Analyze(rr model.RunReport, findings []model.Finding) GuardrailReport {
	buckets := map[string][]CheckerResult{}

	labScores := map[string][]model.Finding{}
	for _, f := range findings {
		labScores[f.Lab] = append(labScores[f.Lab], f)
	}

	for _, ref := range rr.Reflection {
		cat := ClassifyChecker(ref.Lab)
		if cat == "" {
			continue
		}
		cr := CheckerResult{
			Checker: ref.Lab,
			Passed:  ref.Passed,
			Score:   float64(ref.Final) / 10.0,
		}
		if !ref.Passed {
			cr.Details = fmt.Sprintf("score %d/10, %d revisions", ref.Final, ref.Revisions)
		}
		buckets[cat] = append(buckets[cat], cr)
	}

	for lab, ff := range labScores {
		cat := ClassifyChecker(lab)
		if cat == "" {
			continue
		}
		already := false
		for _, cr := range buckets[cat] {
			if cr.Checker == lab {
				already = true
				break
			}
		}
		if already {
			continue
		}
		validated := 0
		for _, f := range ff {
			if f.State == model.StateValidated {
				validated++
			}
		}
		passed := validated == 0
		cr := CheckerResult{
			Checker: lab,
			Passed:  passed,
			Score:   1.0,
		}
		if !passed {
			cr.Score = 0.0
			cr.Details = fmt.Sprintf("%d validated findings", validated)
		}
		buckets[cat] = append(buckets[cat], cr)
	}

	var categories []GuardrailCategory
	totalPassed := 0
	totalTests := 0

	for _, cat := range categoryOrder {
		checkers := buckets[cat]
		if len(checkers) == 0 {
			continue
		}
		sort.Slice(checkers, func(i, j int) bool {
			return checkers[i].Checker < checkers[j].Checker
		})
		passed := 0
		for _, c := range checkers {
			if c.Passed {
				passed++
			}
		}
		rate := 0.0
		if len(checkers) > 0 {
			rate = float64(passed) / float64(len(checkers))
		}
		categories = append(categories, GuardrailCategory{
			Name:       cat,
			PassRate:   rate,
			TotalTests: len(checkers),
			Passed:     passed,
			Failed:     len(checkers) - passed,
			Checkers:   checkers,
		})
		totalPassed += passed
		totalTests += len(checkers)
	}

	overall := 0.0
	if totalTests > 0 {
		overall = float64(totalPassed) / float64(totalTests)
	}

	weakest := ""
	strongest := ""
	minRate := 2.0
	maxRate := -1.0
	for _, c := range categories {
		if c.PassRate < minRate {
			minRate = c.PassRate
			weakest = c.Name
		}
		if c.PassRate > maxRate {
			maxRate = c.PassRate
			strongest = c.Name
		}
	}

	return GuardrailReport{
		Categories:    categories,
		OverallPass:   overall,
		WeakestArea:   weakest,
		StrongestArea: strongest,
	}
}

func FormatText(r GuardrailReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Guardrail Effectiveness Report\n")
	fmt.Fprintf(&b, "  Overall pass rate: %.0f%%\n", r.OverallPass*100)
	if r.StrongestArea != "" {
		fmt.Fprintf(&b, "  Strongest: %s  Weakest: %s\n", r.StrongestArea, r.WeakestArea)
	}
	b.WriteString("\n")

	for _, cat := range r.Categories {
		status := "PASS"
		if cat.PassRate < 0.8 {
			status = "WARN"
		}
		if cat.PassRate < 0.5 {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "  %-20s %3.0f%% (%d/%d) [%s]\n", cat.Name, cat.PassRate*100, cat.Passed, cat.TotalTests, status)
		for _, cr := range cat.Checkers {
			mark := "+"
			if !cr.Passed {
				mark = "-"
			}
			detail := ""
			if cr.Details != "" {
				detail = "  " + cr.Details
			}
			fmt.Fprintf(&b, "    %s %-30s%.2f%s\n", mark, cr.Checker, cr.Score, detail)
		}
	}

	return b.String()
}
