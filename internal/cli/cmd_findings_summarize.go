package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

func init() {
	Register("findings-summarize", "generate executive summary of findings", runFindingsSummarize)
}

type summarySection struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type summarizeResult struct {
	Grade        string           `json:"grade"`
	Score        int              `json:"score"`
	Total        int              `json:"total"`
	BySeverity   map[string]int   `json:"by_severity"`
	ByChecker    map[string]int   `json:"by_checker"`
	TopIssues    []summarySection `json:"top_issues"`
	Highlights   []string         `json:"highlights"`
	Verdict      string           `json:"verdict"`
}

func runFindingsSummarize(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-summarize", stderr)
	input := fs.String("input", "", "path to findings JSON file (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay findings-summarize: --input is required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-summarize: %v\n", err)
		return ExitError
	}

	var findings []map[string]any
	if err := json.Unmarshal(data, &findings); err != nil {
		fmt.Fprintf(stderr, "assay findings-summarize: invalid JSON: %v\n", err)
		return ExitError
	}

	bySev := make(map[string]int)
	byChecker := make(map[string]int)
	issueTypes := make(map[string]int)

	for _, f := range findings {
		sev, _ := f["severity"].(string)
		if sev == "" {
			sev = "unknown"
		}
		bySev[sev]++

		chk, _ := f["checker"].(string)
		if chk == "" {
			chk = "unknown"
		}
		byChecker[chk]++

		cls, _ := f["class"].(string)
		if cls != "" {
			issueTypes[cls]++
		}
	}

	score := summaryScore(bySev)
	grade := summaryGrade(score)

	var topIssues []summarySection
	type kv struct{ k string; v int }
	var sortedTypes []kv
	for k, v := range issueTypes {
		sortedTypes = append(sortedTypes, kv{k, v})
	}
	sort.Slice(sortedTypes, func(i, j int) bool { return sortedTypes[i].v > sortedTypes[j].v })
	for i, st := range sortedTypes {
		if i >= 5 {
			break
		}
		topIssues = append(topIssues, summarySection{Label: st.k, Value: fmt.Sprintf("%d findings", st.v)})
	}

	highlights := buildHighlights(bySev, byChecker, len(findings))
	verdict := buildVerdict(grade, bySev, len(findings))

	result := summarizeResult{
		Grade:      grade,
		Score:      score,
		Total:      len(findings),
		BySeverity: bySev,
		ByChecker:  byChecker,
		TopIssues:  topIssues,
		Highlights: highlights,
		Verdict:    verdict,
	}

	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	} else {
		printExecutiveSummary(stdout, result)
	}
	return ExitPass
}

func summaryScore(bySev map[string]int) int {
	score := 100
	score -= bySev["critical"] * 25
	score -= bySev["high"] * 10
	score -= bySev["medium"] * 5
	score -= bySev["low"] * 2
	if score < 0 {
		score = 0
	}
	return score
}

func summaryGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 60:
		return "C"
	case score >= 40:
		return "D"
	default:
		return "F"
	}
}

func buildHighlights(bySev, byChecker map[string]int, total int) []string {
	var hl []string
	if bySev["critical"] > 0 {
		hl = append(hl, fmt.Sprintf("%d critical findings require immediate attention", bySev["critical"]))
	}
	if len(byChecker) > 3 {
		hl = append(hl, fmt.Sprintf("Issues detected by %d different checkers", len(byChecker)))
	}
	if total == 0 {
		hl = append(hl, "No findings detected — clean scan")
	} else if bySev["critical"] == 0 && bySev["high"] == 0 {
		hl = append(hl, "No critical or high severity findings")
	}
	return hl
}

func buildVerdict(grade string, bySev map[string]int, total int) string {
	if total == 0 {
		return "Clean scan. No security findings detected."
	}
	switch grade {
	case "A":
		return "Minimal risk. Minor findings only — no blockers."
	case "B":
		return "Low risk. Some findings need attention but no critical issues."
	case "C":
		return "Moderate risk. Multiple findings that should be addressed."
	case "D":
		return "High risk. Significant findings requiring remediation."
	default:
		return "Critical risk. Immediate remediation required before deployment."
	}
}

func printExecutiveSummary(w io.Writer, r summarizeResult) {
	fmt.Fprintf(w, "Executive Summary\n")
	fmt.Fprintf(w, "%s\n\n", strings.Repeat("=", 40))
	fmt.Fprintf(w, "Grade: %s (%d/100)\n", r.Grade, r.Score)
	fmt.Fprintf(w, "Total findings: %d\n\n", r.Total)

	fmt.Fprintf(w, "Severity breakdown:\n")
	for _, sev := range []string{"critical", "high", "medium", "low"} {
		if c, ok := r.BySeverity[sev]; ok && c > 0 {
			fmt.Fprintf(w, "  %-10s %d\n", sev, c)
		}
	}

	if len(r.ByChecker) > 0 {
		fmt.Fprintf(w, "\nBy checker:\n")
		for chk, c := range r.ByChecker {
			fmt.Fprintf(w, "  %-20s %d\n", chk, c)
		}
	}

	if len(r.TopIssues) > 0 {
		fmt.Fprintf(w, "\nTop issue types:\n")
		for _, issue := range r.TopIssues {
			fmt.Fprintf(w, "  %-30s %s\n", issue.Label, issue.Value)
		}
	}

	if len(r.Highlights) > 0 {
		fmt.Fprintf(w, "\nHighlights:\n")
		for _, h := range r.Highlights {
			fmt.Fprintf(w, "  - %s\n", h)
		}
	}

	fmt.Fprintf(w, "\nVerdict: %s\n", r.Verdict)
}
