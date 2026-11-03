package coveragemap

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type SecurityArea struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CoverageCell struct {
	Checker  string `json:"checker"`
	Area     string `json:"area"`
	Covered  bool   `json:"covered"`
	Strength string `json:"strength"`
	Evidence int    `json:"evidence"`
}

type CoverageMap struct {
	Checkers []string       `json:"checkers"`
	Areas    []SecurityArea `json:"areas"`
	Cells    []CoverageCell `json:"cells"`
	Gaps     []string       `json:"gaps"`
	Overlaps []string       `json:"overlaps"`
}

var areas = []SecurityArea{
	{Name: "prompt_injection", Description: "Prompt injection and manipulation attacks"},
	{Name: "data_leakage", Description: "Unauthorized data exposure or exfiltration"},
	{Name: "output_safety", Description: "Harmful, toxic or unsafe model outputs"},
	{Name: "tool_use_safety", Description: "Unsafe or unauthorized tool/function calls"},
	{Name: "hallucination", Description: "Fabricated or factually incorrect content"},
	{Name: "bias_fairness", Description: "Biased or discriminatory model behavior"},
	{Name: "token_abuse", Description: "Token limit abuse or resource exhaustion"},
	{Name: "system_prompt_leak", Description: "System prompt extraction or disclosure"},
	{Name: "jailbreak", Description: "Jailbreak and guardrail bypass attempts"},
	{Name: "privacy", Description: "PII exposure or privacy violations"},
}

var areaKeywords = map[string][]string{
	"prompt_injection":   {"injection", "prompt", "inject"},
	"data_leakage":       {"leak", "exfil", "data", "exposure"},
	"output_safety":      {"output", "safety", "harmful", "toxic", "content"},
	"tool_use_safety":    {"tool", "function", "call", "exec"},
	"hallucination":      {"hallucin", "factual", "accuracy", "ground"},
	"bias_fairness":      {"bias", "fair", "discrimin", "equity"},
	"token_abuse":        {"token", "resource", "limit", "exhaust", "dos"},
	"system_prompt_leak": {"system", "prompt_leak", "system_prompt", "disclosure"},
	"jailbreak":          {"jailbreak", "bypass", "guardrail", "escape"},
	"privacy":            {"privacy", "pii", "personal", "gdpr"},
}

func Areas() []SecurityArea { return areas }

func classifyChecker(checker string) []string {
	lower := strings.ToLower(checker)
	var matched []string
	for area, keywords := range areaKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				matched = append(matched, area)
				break
			}
		}
	}
	return matched
}

func classifyFinding(f model.Finding) []string {
	text := strings.ToLower(f.Summary + " " + f.Lab + " " + string(f.Class))
	var matched []string
	for area, keywords := range areaKeywords {
		for _, kw := range keywords {
			if strings.Contains(text, kw) {
				matched = append(matched, area)
				break
			}
		}
	}
	return matched
}

func strengthLabel(count int) string {
	switch {
	case count >= 5:
		return "strong"
	case count >= 2:
		return "moderate"
	case count >= 1:
		return "weak"
	default:
		return "none"
	}
}

func BuildMap(rr model.RunReport, findings []model.Finding) CoverageMap {
	checkerSet := map[string]bool{}
	for _, r := range rr.Reflection {
		checkerSet[r.Reviewer] = true
	}
	for _, f := range findings {
		checkerSet[f.Model.Provider] = true
	}

	checkers := make([]string, 0, len(checkerSet))
	for c := range checkerSet {
		if c != "" {
			checkers = append(checkers, c)
		}
	}
	sort.Strings(checkers)

	type key struct{ checker, area string }
	evidence := map[key]int{}

	for _, c := range checkers {
		for _, a := range classifyChecker(c) {
			evidence[key{c, a}]++
		}
	}
	for _, f := range findings {
		checker := f.Model.Provider
		if checker == "" {
			checker = f.Model.Model
		}
		for _, a := range classifyFinding(f) {
			evidence[key{checker, a}]++
		}
	}

	var cells []CoverageCell
	areaCoverage := map[string]int{}
	for _, c := range checkers {
		for _, a := range areas {
			count := evidence[key{c, a.Name}]
			covered := count > 0
			cells = append(cells, CoverageCell{
				Checker:  c,
				Area:     a.Name,
				Covered:  covered,
				Strength: strengthLabel(count),
				Evidence: count,
			})
			if covered {
				areaCoverage[a.Name]++
			}
		}
	}

	var gaps, overlaps []string
	for _, a := range areas {
		count := areaCoverage[a.Name]
		if count == 0 {
			gaps = append(gaps, a.Name)
		}
		if count >= 3 {
			overlaps = append(overlaps, a.Name)
		}
	}

	return CoverageMap{
		Checkers: checkers,
		Areas:    areas,
		Cells:    cells,
		Gaps:     gaps,
		Overlaps: overlaps,
	}
}

func FormatText(m CoverageMap) string {
	if len(m.Checkers) == 0 {
		return "No checkers found in run data.\n"
	}

	var b strings.Builder

	maxChecker := 7
	for _, c := range m.Checkers {
		if len(c) > maxChecker {
			maxChecker = len(c)
		}
	}

	b.WriteString(fmt.Sprintf("%-*s", maxChecker+2, "Checker"))
	for _, a := range m.Areas {
		short := a.Name
		if len(short) > 12 {
			short = short[:12]
		}
		b.WriteString(fmt.Sprintf(" %-12s", short))
	}
	b.WriteString("\n")
	b.WriteString(strings.Repeat("-", maxChecker+2+13*len(m.Areas)))
	b.WriteString("\n")

	cellMap := map[string]CoverageCell{}
	for _, c := range m.Cells {
		cellMap[c.Checker+"|"+c.Area] = c
	}

	for _, checker := range m.Checkers {
		b.WriteString(fmt.Sprintf("%-*s", maxChecker+2, checker))
		for _, a := range m.Areas {
			cell := cellMap[checker+"|"+a.Name]
			sym := "  -"
			switch cell.Strength {
			case "strong":
				sym = " ++"
			case "moderate":
				sym = "  +"
			case "weak":
				sym = "  ~"
			}
			b.WriteString(fmt.Sprintf(" %-12s", sym))
		}
		b.WriteString("\n")
	}

	b.WriteString("\nLegend: ++ strong, + moderate, ~ weak, - none\n")

	if len(m.Gaps) > 0 {
		b.WriteString(fmt.Sprintf("\nGaps (no coverage): %s\n", strings.Join(m.Gaps, ", ")))
	}
	if len(m.Overlaps) > 0 {
		b.WriteString(fmt.Sprintf("Overlaps (3+ checkers): %s\n", strings.Join(m.Overlaps, ", ")))
	}

	return b.String()
}
