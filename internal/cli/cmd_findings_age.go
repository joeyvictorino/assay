package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

func init() {
	Register("findings-age", "finding age analysis showing persistence across runs", runFindingsAge)
}

type findingAgeEntry struct {
	ID        string `json:"id"`
	Checker   string `json:"checker"`
	Severity  string `json:"severity"`
	FirstSeen string `json:"first_seen"`
	LastSeen  string `json:"last_seen"`
	AgeDays   int    `json:"age_days"`
	RunCount  int    `json:"run_count"`
}

type findingsAgeResult struct {
	Entries    []findingAgeEntry `json:"entries"`
	Total      int               `json:"total"`
	AvgAgeDays float64           `json:"avg_age_days"`
	MaxAgeDays int               `json:"max_age_days"`
	ByAge      map[string]int    `json:"by_age_bucket"`
}

type findingRecord struct {
	ID       string `json:"id"`
	Checker  string `json:"checker"`
	Severity string `json:"severity"`
	RunDate  string `json:"run_date"`
}

func runFindingsAge(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-age", stderr)
	input := fs.String("input", "", "path to findings history JSON file (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay findings-age: --input is required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-age: read input: %v\n", err)
		return ExitError
	}

	var records []findingRecord
	if err := json.Unmarshal(data, &records); err != nil {
		fmt.Fprintf(stderr, "assay findings-age: parse input: %v\n", err)
		return ExitError
	}

	result := buildFindingsAge(records)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	default:
		fmt.Fprint(stdout, formatFindingsAgeText(result))
	}
	return ExitPass
}

func buildFindingsAge(records []findingRecord) findingsAgeResult {
	type findingInfo struct {
		checker  string
		severity string
		first    string
		last     string
		runs     int
	}

	byID := map[string]*findingInfo{}
	for _, r := range records {
		fi, ok := byID[r.ID]
		if !ok {
			fi = &findingInfo{checker: r.Checker, severity: r.Severity, first: r.RunDate, last: r.RunDate}
			byID[r.ID] = fi
		}
		fi.runs++
		if r.RunDate < fi.first {
			fi.first = r.RunDate
		}
		if r.RunDate > fi.last {
			fi.last = r.RunDate
		}
	}

	var entries []findingAgeEntry
	totalDays := 0
	maxDays := 0
	buckets := map[string]int{}

	for id, fi := range byID {
		days := daysBetween(fi.first, fi.last)
		entries = append(entries, findingAgeEntry{
			ID:        id,
			Checker:   fi.checker,
			Severity:  fi.severity,
			FirstSeen: fi.first,
			LastSeen:  fi.last,
			AgeDays:   days,
			RunCount:  fi.runs,
		})
		totalDays += days
		if days > maxDays {
			maxDays = days
		}
		bucket := ageBucket(days)
		buckets[bucket]++
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].AgeDays > entries[j].AgeDays
	})

	avg := 0.0
	if len(entries) > 0 {
		avg = float64(totalDays) / float64(len(entries))
	}

	return findingsAgeResult{
		Entries:    entries,
		Total:      len(entries),
		AvgAgeDays: avg,
		MaxAgeDays: maxDays,
		ByAge:      buckets,
	}
}

func daysBetween(a, b string) int {
	ta, err1 := time.Parse("2006-01-02", a)
	tb, err2 := time.Parse("2006-01-02", b)
	if err1 != nil || err2 != nil {
		return 0
	}
	d := tb.Sub(ta)
	if d < 0 {
		d = -d
	}
	return int(d.Hours() / 24)
}

func ageBucket(days int) string {
	switch {
	case days == 0:
		return "new"
	case days <= 7:
		return "1_week"
	case days <= 30:
		return "1_month"
	case days <= 90:
		return "3_months"
	default:
		return "older"
	}
}

func formatFindingsAgeText(r findingsAgeResult) string {
	var b strings.Builder
	b.WriteString("Finding Age Analysis\n")
	b.WriteString("========================================\n")
	fmt.Fprintf(&b, "Total unique findings: %d\n", r.Total)
	fmt.Fprintf(&b, "Average age: %.1f days\n", r.AvgAgeDays)
	fmt.Fprintf(&b, "Max age:     %d days\n\n", r.MaxAgeDays)

	if len(r.ByAge) > 0 {
		b.WriteString("Age distribution:\n")
		for _, bucket := range []string{"new", "1_week", "1_month", "3_months", "older"} {
			if count, ok := r.ByAge[bucket]; ok {
				fmt.Fprintf(&b, "  %-12s %d\n", bucket, count)
			}
		}
		b.WriteString("\n")
	}

	if len(r.Entries) > 0 {
		b.WriteString("Oldest findings:\n")
		limit := len(r.Entries)
		if limit > 10 {
			limit = 10
		}
		for _, e := range r.Entries[:limit] {
			fmt.Fprintf(&b, "  [%dd] %-15s %-10s %s\n", e.AgeDays, e.Checker, e.Severity, e.ID)
		}
		b.WriteString("\n")
	}

	return b.String()
}
