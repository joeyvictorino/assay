package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

type ConsolidatedReport struct {
	TotalRuns    int              `json:"total_runs"`
	PassCount    int              `json:"pass_count"`
	BlockedCount int              `json:"blocked_count"`
	ErrorCount   int              `json:"error_count"`
	FirstRun     time.Time        `json:"first_run"`
	LastRun      time.Time        `json:"last_run"`
	Trend        string           `json:"trend"`
	Runs         []RunSummaryItem `json:"runs"`
}

type RunSummaryItem struct {
	RunID      string    `json:"run_id"`
	Started    time.Time `json:"started"`
	Verdict    string    `json:"verdict"`
	Models     int       `json:"models"`
	SpentUSD   float64   `json:"spent_usd"`
	PolicyHash string    `json:"policy_hash,omitempty"`
}

func LoadRuns(dir string) ([]model.RunReport, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var runs []model.RunReport
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", e.Name(), err)
		}
		var r model.RunReport
		if err := json.Unmarshal(raw, &r); err != nil {
			continue
		}
		if r.RunID == "" {
			continue
		}
		runs = append(runs, r)
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].Started.Before(runs[j].Started)
	})
	return runs, nil
}

func Consolidate(runs []model.RunReport) ConsolidatedReport {
	cr := ConsolidatedReport{TotalRuns: len(runs)}
	if len(runs) == 0 {
		cr.Trend = "none"
		return cr
	}

	for _, r := range runs {
		item := RunSummaryItem{
			RunID:    r.RunID,
			Started:  r.Started,
			Verdict:  r.Verdict,
			Models:   len(r.Models),
			SpentUSD: r.SpentUSD,
		}
		cr.Runs = append(cr.Runs, item)

		switch r.Verdict {
		case "PASS":
			cr.PassCount++
		case "BLOCKED":
			cr.BlockedCount++
		default:
			cr.ErrorCount++
		}
	}

	cr.FirstRun = runs[0].Started
	cr.LastRun = runs[len(runs)-1].Started

	if len(runs) < 2 {
		cr.Trend = "insufficient_data"
	} else {
		mid := len(runs) / 2
		firstHalfPass := 0
		for _, r := range runs[:mid] {
			if r.Verdict == "PASS" {
				firstHalfPass++
			}
		}
		secondHalfPass := 0
		for _, r := range runs[mid:] {
			if r.Verdict == "PASS" {
				secondHalfPass++
			}
		}
		firstRate := float64(firstHalfPass) / float64(mid)
		secondRate := float64(secondHalfPass) / float64(len(runs)-mid)
		if secondRate > firstRate+0.1 {
			cr.Trend = "improving"
		} else if secondRate < firstRate-0.1 {
			cr.Trend = "degrading"
		} else {
			cr.Trend = "stable"
		}
	}

	return cr
}

func FormatConsolidateText(cr ConsolidatedReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Consolidated Report: %d runs\n", cr.TotalRuns)
	if cr.TotalRuns == 0 {
		fmt.Fprintln(&b, "  No runs found.")
		return b.String()
	}
	fmt.Fprintf(&b, "  Date range: %s to %s\n", cr.FirstRun.Format("2006-01-02"), cr.LastRun.Format("2006-01-02"))
	fmt.Fprintf(&b, "  Pass: %d  Blocked: %d  Error: %d\n", cr.PassCount, cr.BlockedCount, cr.ErrorCount)
	fmt.Fprintf(&b, "  Trend: %s\n\n", cr.Trend)

	fmt.Fprintln(&b, "  Run ID                              Started             Verdict  Models  Spent")
	fmt.Fprintln(&b, "  ------------------------------------  -------------------  -------  ------  -------")
	for _, r := range cr.Runs {
		id := r.RunID
		if len(id) > 36 {
			id = id[:36]
		}
		fmt.Fprintf(&b, "  %-36s  %-19s  %-7s  %6d  $%.2f\n",
			id, r.Started.Format("2006-01-02 15:04"), r.Verdict, r.Models, r.SpentUSD)
	}
	return b.String()
}

func FormatConsolidateHTML(cr ConsolidatedReport) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html><html><head><meta charset=\"utf-8\"><title>Assay Consolidated Report</title>")
	b.WriteString("<style>body{font-family:sans-serif;margin:2em}table{border-collapse:collapse;width:100%}")
	b.WriteString("th,td{border:1px solid #ddd;padding:8px;text-align:left}th{background:#f5f5f5}")
	b.WriteString(".pass{color:#2a7}. blocked{color:#c42}.trend{font-size:1.2em;font-weight:bold}</style></head><body>")
	fmt.Fprintf(&b, "<h1>Assay Consolidated Report</h1>")
	fmt.Fprintf(&b, "<p>%d runs from %s to %s</p>",
		cr.TotalRuns, cr.FirstRun.Format("2006-01-02"), cr.LastRun.Format("2006-01-02"))
	fmt.Fprintf(&b, "<p>Pass: %d | Blocked: %d | Error: %d</p>", cr.PassCount, cr.BlockedCount, cr.ErrorCount)
	fmt.Fprintf(&b, "<p class=\"trend\">Trend: %s</p>", cr.Trend)

	b.WriteString("<table><thead><tr><th>Run ID</th><th>Started</th><th>Verdict</th><th>Models</th><th>Spent</th></tr></thead><tbody>")
	for _, r := range cr.Runs {
		cls := ""
		if r.Verdict == "PASS" {
			cls = "pass"
		} else if r.Verdict == "BLOCKED" {
			cls = "blocked"
		}
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td class=\"%s\">%s</td><td>%d</td><td>$%.2f</td></tr>",
			r.RunID, r.Started.Format("2006-01-02 15:04"), cls, r.Verdict, r.Models, r.SpentUSD)
	}
	b.WriteString("</tbody></table></body></html>")
	return b.String()
}

