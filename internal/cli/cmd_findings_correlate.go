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
	Register("findings-correlate", "correlate findings across checkers by shared attributes", runFindingsCorrelate)
}

type correlation struct {
	Key      string   `json:"key"`
	Value    string   `json:"value"`
	Checkers []string `json:"checkers"`
	Count    int      `json:"count"`
	Findings int      `json:"findings"`
}

type correlateResult struct {
	Total        int           `json:"total_findings"`
	Correlations []correlation `json:"correlations"`
	MultiChecker int           `json:"multi_checker_count"`
}

func runFindingsCorrelate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-correlate", stderr)
	input := fs.String("input", "", "path to findings JSON file (required)")
	key := fs.String("key", "class", "field to correlate by: class, file, severity, message")
	minCheckers := fs.Int("min-checkers", 2, "minimum distinct checkers to report")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay findings-correlate: --input is required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-correlate: %v\n", err)
		return ExitError
	}

	var findings []map[string]any
	if err := json.Unmarshal(data, &findings); err != nil {
		fmt.Fprintf(stderr, "assay findings-correlate: invalid JSON: %v\n", err)
		return ExitError
	}

	groups := make(map[string]map[string]int) // value -> checker -> count
	for _, f := range findings {
		val, _ := f[*key].(string)
		if val == "" {
			val = "(empty)"
		}
		chk, _ := f["checker"].(string)
		if chk == "" {
			chk = "unknown"
		}
		if groups[val] == nil {
			groups[val] = make(map[string]int)
		}
		groups[val][chk]++
	}

	var correlations []correlation
	multiCount := 0
	for val, checkers := range groups {
		if len(checkers) < *minCheckers {
			continue
		}
		var names []string
		total := 0
		for chk, c := range checkers {
			names = append(names, chk)
			total += c
		}
		sort.Strings(names)
		correlations = append(correlations, correlation{
			Key:      *key,
			Value:    val,
			Checkers: names,
			Count:    len(names),
			Findings: total,
		})
		multiCount++
	}

	sort.Slice(correlations, func(i, j int) bool {
		if correlations[i].Count != correlations[j].Count {
			return correlations[i].Count > correlations[j].Count
		}
		return correlations[i].Findings > correlations[j].Findings
	})

	result := correlateResult{
		Total:        len(findings),
		Correlations: correlations,
		MultiChecker: multiCount,
	}

	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	} else {
		fmt.Fprintf(stdout, "Findings Correlation (by %s, min %d checkers)\n\n", *key, *minCheckers)
		if len(correlations) == 0 {
			fmt.Fprintln(stdout, "No cross-checker correlations found.")
		} else {
			fmt.Fprintf(stdout, "%-30s %8s %8s  %s\n", "Value", "Checkers", "Findings", "Checker Names")
			fmt.Fprintf(stdout, "%s\n", strings.Repeat("-", 80))
			for _, c := range correlations {
				fmt.Fprintf(stdout, "%-30s %8d %8d  %s\n", truncate(c.Value, 30), c.Count, c.Findings, strings.Join(c.Checkers, ", "))
			}
		}
		fmt.Fprintf(stdout, "\nMulti-checker correlations: %d\n", multiCount)
	}
	return ExitPass
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
