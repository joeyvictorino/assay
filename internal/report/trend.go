package report

import (
	"encoding/json"
	"math"
	"sort"

	"github.com/joeyvictorino/assay/internal/model"
)

// TrendReport summarises directional changes across a series of runs.
type TrendReport struct {
	RunCount       int              `json:"run_count"`
	Direction      string           `json:"direction"`
	PassStreak     int              `json:"pass_streak"`
	FailStreak     int              `json:"fail_streak"`
	CostTrend      string           `json:"cost_trend"`
	ModelStability float64          `json:"model_stability"`
	Runs           []TrendRunEntry  `json:"runs"`
	ModelPresence  map[string]int   `json:"model_presence"`
}

// TrendRunEntry is one point in the trend series.
type TrendRunEntry struct {
	RunID     string  `json:"run_id"`
	Date      string  `json:"date"`
	Verdict   string  `json:"verdict"`
	Validated int     `json:"validated"`
	Theorized int     `json:"theorized"`
	Refusals  int     `json:"refusals"`
	TotalCost float64 `json:"total_cost"`
	Models    int     `json:"models"`
}

// BuildTrend analyses a chronological series of runs and returns a trend report.
func BuildTrend(runs []model.RunReport) TrendReport {
	rows := Rows(runs)

	tr := TrendReport{
		RunCount:      len(rows),
		Direction:     "stable",
		ModelPresence: map[string]int{},
	}
	if len(rows) == 0 {
		return tr
	}

	entries := make([]TrendRunEntry, 0, len(rows))
	for _, r := range rows {
		e := TrendRunEntry{
			RunID:   r.RunID,
			Date:    r.Date,
			Verdict: r.Verdict,
		}
		var cost float64
		for _, m := range r.Models {
			e.Validated += m.FindingsValidated
			e.Theorized += m.FindingsTheorized
			e.Refusals += m.Refusals
			cost += m.CostUSD
			tr.ModelPresence[m.Model]++
		}
		e.TotalCost = math.Round(cost*10000) / 10000
		e.Models = len(r.Models)
		entries = append(entries, e)
	}
	tr.Runs = entries

	// Finding direction: compare first half average to second half average.
	if len(entries) >= 2 {
		mid := len(entries) / 2
		firstAvg := avgValidated(entries[:mid])
		secondAvg := avgValidated(entries[mid:])
		if secondAvg > firstAvg*1.1 {
			tr.Direction = "increasing"
		} else if secondAvg < firstAvg*0.9 {
			tr.Direction = "decreasing"
		}
	}

	// Cost trend.
	tr.CostTrend = "stable"
	if len(entries) >= 2 {
		firstCost := entries[0].TotalCost
		lastCost := entries[len(entries)-1].TotalCost
		if firstCost > 0 {
			ratio := lastCost / firstCost
			if ratio > 1.1 {
				tr.CostTrend = "increasing"
			} else if ratio < 0.9 {
				tr.CostTrend = "decreasing"
			}
		}
	}

	// Pass/fail streaks from the end.
	tr.PassStreak, tr.FailStreak = streaks(entries)

	// Model stability: fraction of runs that use the most common model set.
	tr.ModelStability = modelStability(entries, rows)

	return tr
}

// TrendJSON encodes a TrendReport as indented JSON.
func TrendJSON(tr TrendReport) (string, error) {
	b, err := json.MarshalIndent(tr, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}

func avgValidated(entries []TrendRunEntry) float64 {
	if len(entries) == 0 {
		return 0
	}
	sum := 0
	for _, e := range entries {
		sum += e.Validated
	}
	return float64(sum) / float64(len(entries))
}

func streaks(entries []TrendRunEntry) (passStreak, failStreak int) {
	if len(entries) == 0 {
		return 0, 0
	}
	last := entries[len(entries)-1].Verdict
	if last == "pass" || last == "" {
		for i := len(entries) - 1; i >= 0; i-- {
			v := entries[i].Verdict
			if v != "pass" && v != "" {
				break
			}
			passStreak++
		}
	} else {
		for i := len(entries) - 1; i >= 0; i-- {
			v := entries[i].Verdict
			if v == "pass" || v == "" {
				break
			}
			failStreak++
		}
	}
	return
}

func modelStability(entries []TrendRunEntry, rows []DashboardRow) float64 {
	if len(rows) == 0 {
		return 0
	}
	type modelSet struct {
		key   string
		count int
	}
	setCounts := map[string]int{}
	for _, r := range rows {
		names := make([]string, 0, len(r.Models))
		for _, m := range r.Models {
			names = append(names, m.Model)
		}
		sort.Strings(names)
		key := ""
		for i, n := range names {
			if i > 0 {
				key += ","
			}
			key += n
		}
		setCounts[key]++
	}
	maxCount := 0
	for _, c := range setCounts {
		if c > maxCount {
			maxCount = c
		}
	}
	return math.Round(float64(maxCount)/float64(len(rows))*100) / 100
}
