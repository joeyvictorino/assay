package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func init() {
	Register("findings-trend", "analyze finding trends across multiple run files", runFindingsTrend)
}

type trendPoint struct {
	Source   string         `json:"source"`
	Total   int            `json:"total"`
	BySev   map[string]int `json:"by_severity"`
	ByCheck map[string]int `json:"by_checker"`
}

type trendResult struct {
	Points     []trendPoint   `json:"points"`
	TotalRuns  int            `json:"total_runs"`
	SevTrend   map[string][]int `json:"severity_trend"`
	CheckTrend map[string][]int `json:"checker_trend"`
}

func runFindingsTrend(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-trend", stderr)
	inputDir := fs.String("dir", "", "directory containing findings JSON files (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *inputDir == "" {
		fmt.Fprintln(stderr, "assay findings-trend: --dir is required")
		return ExitError
	}

	entries, err := os.ReadDir(*inputDir)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-trend: %v\n", err)
		return ExitError
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	if len(files) == 0 {
		fmt.Fprintln(stderr, "assay findings-trend: no .json files found in directory")
		return ExitError
	}

	var points []trendPoint
	sevTrend := make(map[string][]int)
	checkTrend := make(map[string][]int)

	for _, fname := range files {
		data, err := os.ReadFile(filepath.Join(*inputDir, fname))
		if err != nil {
			fmt.Fprintf(stderr, "assay findings-trend: skip %s: %v\n", fname, err)
			continue
		}
		var findings []map[string]any
		if err := json.Unmarshal(data, &findings); err != nil {
			fmt.Fprintf(stderr, "assay findings-trend: skip %s: %v\n", fname, err)
			continue
		}

		bySev := make(map[string]int)
		byCheck := make(map[string]int)
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
			byCheck[chk]++
		}

		pt := trendPoint{
			Source:   fname,
			Total:   len(findings),
			BySev:   bySev,
			ByCheck: byCheck,
		}
		points = append(points, pt)

		for s, c := range bySev {
			sevTrend[s] = append(sevTrend[s], c)
		}
		for ch, c := range byCheck {
			checkTrend[ch] = append(checkTrend[ch], c)
		}
	}

	result := trendResult{
		Points:     points,
		TotalRuns:  len(points),
		SevTrend:   sevTrend,
		CheckTrend: checkTrend,
	}

	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	} else {
		fmt.Fprintf(stdout, "Findings Trend Analysis\n")
		fmt.Fprintf(stdout, "Runs analyzed: %d\n\n", result.TotalRuns)

		fmt.Fprintf(stdout, "%-30s %8s %8s\n", "Source", "Total", "Sev")
		fmt.Fprintf(stdout, "%s\n", strings.Repeat("-", 52))
		for _, pt := range points {
			sevStr := formatSevMap(pt.BySev)
			fmt.Fprintf(stdout, "%-30s %8d %s\n", pt.Source, pt.Total, sevStr)
		}

		if len(sevTrend) > 0 {
			fmt.Fprintf(stdout, "\nSeverity Trends:\n")
			for sev, vals := range sevTrend {
				fmt.Fprintf(stdout, "  %s: %v\n", sev, vals)
			}
		}
	}
	return ExitPass
}

func formatSevMap(m map[string]int) string {
	var parts []string
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s=%d", k, v))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}
