package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

func init() {
	Register("findings-stats", "aggregate statistics about findings (counts by severity, checker, class)", runFindingsStats)
}

type findingsStatsResult struct {
	Total      int            `json:"total"`
	BySeverity map[string]int `json:"by_severity"`
	ByChecker  map[string]int `json:"by_checker"`
	ByClass    map[string]int `json:"by_class"`
	ByState    map[string]int `json:"by_state"`
}

func runFindingsStats(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-stats", stderr)
	input := fs.String("input", "", "path to findings JSON file (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay findings-stats: --input is required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-stats: %v\n", err)
		return ExitError
	}

	var findings []map[string]any
	if err := json.Unmarshal(data, &findings); err != nil {
		fmt.Fprintf(stderr, "assay findings-stats: invalid JSON: %v\n", err)
		return ExitError
	}

	res := findingsStatsResult{
		Total:      len(findings),
		BySeverity: make(map[string]int),
		ByChecker:  make(map[string]int),
		ByClass:    make(map[string]int),
		ByState:    make(map[string]int),
	}

	for _, f := range findings {
		if s, ok := f["severity"].(string); ok && s != "" {
			res.BySeverity[s]++
		}
		if c, ok := f["checker"].(string); ok && c != "" {
			res.ByChecker[c]++
		}
		if cl, ok := f["class"].(string); ok && cl != "" {
			res.ByClass[cl]++
		}
		if st, ok := f["state"].(string); ok && st != "" {
			res.ByState[st]++
		}
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	case "text":
		fmt.Fprintf(stdout, "Findings Statistics (%d total):\n\n", res.Total)
		printSection(stdout, "Severity", res.BySeverity)
		printSection(stdout, "Checker", res.ByChecker)
		printSection(stdout, "Class", res.ByClass)
		printSection(stdout, "State", res.ByState)
	default:
		fmt.Fprintf(stderr, "assay findings-stats: unknown format %q\n", *format)
		return ExitError
	}
	return ExitPass
}

func printSection(w io.Writer, label string, m map[string]int) {
	if len(m) == 0 {
		return
	}
	fmt.Fprintf(w, "  By %s:\n", label)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "    %-25s %d\n", k, m[k])
	}
	fmt.Fprintln(w)
}
