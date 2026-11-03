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
	Register("findings-heatmap", "checker-vs-severity heatmap grid from findings", runFindingsHeatmap)
}

type heatmapCell struct {
	Checker  string `json:"checker"`
	Severity string `json:"severity"`
	Count    int    `json:"count"`
}

type heatmapResult struct {
	Grid       map[string]map[string]int `json:"grid"`
	Cells      []heatmapCell             `json:"cells"`
	Checkers   []string                  `json:"checkers"`
	Severities []string                  `json:"severities"`
	Total      int                       `json:"total"`
}

func runFindingsHeatmap(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-heatmap", stderr)
	input := fs.String("input", "", "path to findings JSON file (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay findings-heatmap: --input is required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-heatmap: %v\n", err)
		return ExitError
	}

	var findings []map[string]any
	if err := json.Unmarshal(data, &findings); err != nil {
		fmt.Fprintf(stderr, "assay findings-heatmap: invalid JSON: %v\n", err)
		return ExitError
	}

	res := buildHeatmap(findings)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	case "text":
		formatHeatmapText(stdout, res)
	default:
		fmt.Fprintf(stderr, "assay findings-heatmap: unknown format %q\n", *format)
		return ExitError
	}
	return ExitPass
}

func buildHeatmap(findings []map[string]any) heatmapResult {
	grid := make(map[string]map[string]int)
	checkerSet := make(map[string]bool)
	sevSet := make(map[string]bool)

	for _, f := range findings {
		checker, _ := f["checker"].(string)
		severity, _ := f["severity"].(string)
		if checker == "" {
			checker = "(unknown)"
		}
		if severity == "" {
			severity = "(unknown)"
		}
		checkerSet[checker] = true
		sevSet[severity] = true
		if grid[checker] == nil {
			grid[checker] = make(map[string]int)
		}
		grid[checker][severity]++
	}

	checkers := make([]string, 0, len(checkerSet))
	for c := range checkerSet {
		checkers = append(checkers, c)
	}
	sort.Strings(checkers)

	severityOrder := []string{"critical", "high", "medium", "low", "info"}
	severities := make([]string, 0)
	seen := make(map[string]bool)
	for _, s := range severityOrder {
		if sevSet[s] {
			severities = append(severities, s)
			seen[s] = true
		}
	}
	for s := range sevSet {
		if !seen[s] {
			severities = append(severities, s)
		}
	}

	cells := make([]heatmapCell, 0)
	for _, c := range checkers {
		for _, s := range severities {
			if count := grid[c][s]; count > 0 {
				cells = append(cells, heatmapCell{Checker: c, Severity: s, Count: count})
			}
		}
	}

	return heatmapResult{
		Grid:       grid,
		Cells:      cells,
		Checkers:   checkers,
		Severities: severities,
		Total:      len(findings),
	}
}

func formatHeatmapText(w io.Writer, res heatmapResult) {
	if res.Total == 0 {
		fmt.Fprintln(w, "No findings.")
		return
	}

	fmt.Fprintf(w, "Findings Heatmap (%d total)\n\n", res.Total)

	colWidth := 10
	header := fmt.Sprintf("%-25s", "Checker")
	for _, s := range res.Severities {
		header += fmt.Sprintf(" %*s", colWidth, s)
	}
	header += fmt.Sprintf(" %*s", colWidth, "TOTAL")
	fmt.Fprintln(w, header)
	fmt.Fprintln(w, strings.Repeat("-", len(header)))

	for _, c := range res.Checkers {
		row := fmt.Sprintf("%-25s", c)
		total := 0
		for _, s := range res.Severities {
			count := res.Grid[c][s]
			total += count
			if count > 0 {
				row += fmt.Sprintf(" %*d", colWidth, count)
			} else {
				row += fmt.Sprintf(" %*s", colWidth, ".")
			}
		}
		row += fmt.Sprintf(" %*d", colWidth, total)
		fmt.Fprintln(w, row)
	}

	fmt.Fprintln(w, strings.Repeat("-", len(header)))
	totals := fmt.Sprintf("%-25s", "TOTAL")
	grandTotal := 0
	for _, s := range res.Severities {
		colTotal := 0
		for _, c := range res.Checkers {
			colTotal += res.Grid[c][s]
		}
		grandTotal += colTotal
		totals += fmt.Sprintf(" %*d", colWidth, colTotal)
	}
	totals += fmt.Sprintf(" %*d", colWidth, grandTotal)
	fmt.Fprintln(w, totals)
}
