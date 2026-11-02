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
	Register("findings-owner", "finding ownership attribution by checker and model", runFindingsOwner)
}

type ownerFinding struct {
	Checker  string `json:"checker"`
	Model    string `json:"model"`
	Severity string `json:"severity"`
	Class    string `json:"class"`
}

type ownerEntry struct {
	Owner      string         `json:"owner"`
	Findings   int            `json:"findings"`
	BySeverity map[string]int `json:"by_severity"`
}

type findingsOwnerResult struct {
	ByChecker []ownerEntry `json:"by_checker"`
	ByModel   []ownerEntry `json:"by_model"`
	Total     int          `json:"total"`
}

func runFindingsOwner(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-owner", stderr)
	input := fs.String("input", "", "path to findings JSON file (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay findings-owner: --input is required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-owner: read input: %v\n", err)
		return ExitError
	}

	var findings []ownerFinding
	if err := json.Unmarshal(data, &findings); err != nil {
		fmt.Fprintf(stderr, "assay findings-owner: parse input: %v\n", err)
		return ExitError
	}

	result := buildFindingsOwner(findings)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	default:
		fmt.Fprint(stdout, formatFindingsOwnerText(result))
	}
	return ExitPass
}

func buildFindingsOwner(findings []ownerFinding) findingsOwnerResult {
	checkerMap := map[string]*ownerEntry{}
	modelMap := map[string]*ownerEntry{}

	for _, f := range findings {
		checker := f.Checker
		if checker == "" {
			checker = "unknown"
		}
		ce, ok := checkerMap[checker]
		if !ok {
			ce = &ownerEntry{Owner: checker, BySeverity: map[string]int{}}
			checkerMap[checker] = ce
		}
		ce.Findings++
		ce.BySeverity[f.Severity]++

		model := f.Model
		if model == "" {
			model = "unknown"
		}
		me, ok := modelMap[model]
		if !ok {
			me = &ownerEntry{Owner: model, BySeverity: map[string]int{}}
			modelMap[model] = me
		}
		me.Findings++
		me.BySeverity[f.Severity]++
	}

	byChecker := mapToSlice(checkerMap)
	byModel := mapToSlice(modelMap)

	return findingsOwnerResult{
		ByChecker: byChecker,
		ByModel:   byModel,
		Total:     len(findings),
	}
}

func mapToSlice(m map[string]*ownerEntry) []ownerEntry {
	var s []ownerEntry
	for _, e := range m {
		s = append(s, *e)
	}
	sort.Slice(s, func(i, j int) bool {
		return s[i].Findings > s[j].Findings
	})
	return s
}

func formatFindingsOwnerText(r findingsOwnerResult) string {
	var b strings.Builder
	b.WriteString("Finding Ownership Attribution\n")
	b.WriteString("========================================\n")
	fmt.Fprintf(&b, "Total findings: %d\n\n", r.Total)

	if len(r.ByChecker) > 0 {
		b.WriteString("By checker:\n")
		fmt.Fprintf(&b, "  %-25s %8s   %s\n", "Checker", "Findings", "Severity breakdown")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, e := range r.ByChecker {
			sevs := formatOwnerSevMap(e.BySeverity)
			fmt.Fprintf(&b, "  %-25s %8d   %s\n", e.Owner, e.Findings, sevs)
		}
		b.WriteString("\n")
	}

	if len(r.ByModel) > 0 {
		b.WriteString("By model:\n")
		fmt.Fprintf(&b, "  %-25s %8s   %s\n", "Model", "Findings", "Severity breakdown")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, e := range r.ByModel {
			sevs := formatOwnerSevMap(e.BySeverity)
			fmt.Fprintf(&b, "  %-25s %8d   %s\n", e.Owner, e.Findings, sevs)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func formatOwnerSevMap(m map[string]int) string {
	var parts []string
	for _, sev := range []string{"critical", "high", "medium", "low", "info"} {
		if c, ok := m[sev]; ok && c > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", sev, c))
		}
	}
	for k, v := range m {
		if k != "critical" && k != "high" && k != "medium" && k != "low" && k != "info" && v > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", k, v))
		}
	}
	return strings.Join(parts, ", ")
}
