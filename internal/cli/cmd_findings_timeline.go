package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func init() {
	Register("findings-timeline", "show temporal analysis of findings across dated run files", runFindingsTimeline)
}

type timelineEntry struct {
	Date     string         `json:"date"`
	Source   string         `json:"source"`
	Total    int            `json:"total"`
	New      int            `json:"new"`
	Resolved int            `json:"resolved"`
	BySev    map[string]int `json:"by_severity"`
}

type timelineResult struct {
	Entries      []timelineEntry `json:"entries"`
	TotalRuns    int             `json:"total_runs"`
	FirstSeen    string          `json:"first_seen"`
	LastSeen     string          `json:"last_seen"`
	NetChange    int             `json:"net_change"`
	SevOverTime  map[string][]int `json:"severity_over_time"`
}

func runFindingsTimeline(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-timeline", stderr)
	inputDir := fs.String("dir", "", "directory containing dated findings JSON files (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *inputDir == "" {
		fmt.Fprintln(stderr, "assay findings-timeline: --dir is required")
		return ExitError
	}

	dirEntries, err := os.ReadDir(*inputDir)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-timeline: %v\n", err)
		return ExitError
	}

	var files []string
	for _, e := range dirEntries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	if len(files) == 0 {
		fmt.Fprintln(stderr, "assay findings-timeline: no .json files found")
		return ExitError
	}

	var entries []timelineEntry
	sevOverTime := make(map[string][]int)
	var prevKeys map[string]bool

	for _, fname := range files {
		data, err := os.ReadFile(filepath.Join(*inputDir, fname))
		if err != nil {
			fmt.Fprintf(stderr, "assay findings-timeline: skip %s: %v\n", fname, err)
			continue
		}
		var findings []map[string]any
		if err := json.Unmarshal(data, &findings); err != nil {
			fmt.Fprintf(stderr, "assay findings-timeline: skip %s: %v\n", fname, err)
			continue
		}

		bySev := make(map[string]int)
		curKeys := make(map[string]bool)
		for _, f := range findings {
			sev, _ := f["severity"].(string)
			if sev == "" {
				sev = "unknown"
			}
			bySev[sev]++

			key := findingKey(f)
			curKeys[key] = true
		}

		newCount := 0
		resolvedCount := 0
		if prevKeys != nil {
			for k := range curKeys {
				if !prevKeys[k] {
					newCount++
				}
			}
			for k := range prevKeys {
				if !curKeys[k] {
					resolvedCount++
				}
			}
		}

		date := extractDate(fname)

		entry := timelineEntry{
			Date:     date,
			Source:   fname,
			Total:    len(findings),
			New:      newCount,
			Resolved: resolvedCount,
			BySev:    bySev,
		}
		entries = append(entries, entry)

		for s, c := range bySev {
			sevOverTime[s] = append(sevOverTime[s], c)
		}

		prevKeys = curKeys
	}

	if len(entries) == 0 {
		fmt.Fprintln(stderr, "assay findings-timeline: no valid files parsed")
		return ExitError
	}

	netChange := entries[len(entries)-1].Total - entries[0].Total

	result := timelineResult{
		Entries:     entries,
		TotalRuns:   len(entries),
		FirstSeen:   entries[0].Date,
		LastSeen:    entries[len(entries)-1].Date,
		NetChange:   netChange,
		SevOverTime: sevOverTime,
	}

	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	} else {
		fmt.Fprintf(stdout, "Findings Timeline (%d runs, %s to %s)\n\n", result.TotalRuns, result.FirstSeen, result.LastSeen)
		fmt.Fprintf(stdout, "%-12s %-25s %6s %6s %8s %s\n", "Date", "Source", "Total", "New", "Resolved", "Sev")
		fmt.Fprintf(stdout, "%s\n", strings.Repeat("-", 80))
		for _, e := range entries {
			sevStr := formatTimelineSev(e.BySev)
			fmt.Fprintf(stdout, "%-12s %-25s %6d %+6d %8d %s\n", e.Date, e.Source, e.Total, e.New, e.Resolved, sevStr)
		}
		fmt.Fprintf(stdout, "\nNet change: %+d findings\n", netChange)

		if len(sevOverTime) > 0 {
			fmt.Fprintf(stdout, "\nSeverity over time:\n")
			for _, sev := range []string{"critical", "high", "medium", "low"} {
				if vals, ok := sevOverTime[sev]; ok {
					fmt.Fprintf(stdout, "  %-10s %v\n", sev, vals)
				}
			}
		}
	}
	return ExitPass
}

func findingKey(f map[string]any) string {
	chk, _ := f["checker"].(string)
	cls, _ := f["class"].(string)
	msg, _ := f["message"].(string)
	return chk + "|" + cls + "|" + msg
}

func extractDate(filename string) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	for _, layout := range []string{"2006-01-02", "20060102", "2006-01-02T15:04:05"} {
		if _, err := time.Parse(layout, base); err == nil {
			return base
		}
	}
	parts := strings.Split(base, "_")
	for _, p := range parts {
		for _, layout := range []string{"2006-01-02", "20060102"} {
			if _, err := time.Parse(layout, p); err == nil {
				return p
			}
		}
	}
	return base
}

func formatTimelineSev(m map[string]int) string {
	var parts []string
	for _, sev := range []string{"critical", "high", "medium", "low"} {
		if c, ok := m[sev]; ok && c > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", sev[:1], c))
		}
	}
	return strings.Join(parts, " ")
}
