package consistency

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type FindingPresence struct {
	Key       string  `json:"key"`
	Class     string  `json:"class"`
	Severity  string  `json:"severity"`
	Summary   string  `json:"summary"`
	RunCount  int     `json:"run_count"`
	TotalRuns int     `json:"total_runs"`
	Rate      float64 `json:"rate"`
	Status    string  `json:"status"`
}

type ConsistencyReport struct {
	TotalRuns       int               `json:"total_runs"`
	TotalFindings   int               `json:"total_findings"`
	UniqueFindings  int               `json:"unique_findings"`
	StableFindings  int               `json:"stable_findings"`
	FlakyFindings   int               `json:"flaky_findings"`
	MissingFindings int               `json:"missing_findings"`
	OverallScore    float64           `json:"overall_score"`
	Threshold       float64           `json:"threshold"`
	Findings        []FindingPresence `json:"findings"`
}

func findingKey(f model.Finding) string {
	return string(f.Class) + "|" + string(f.Severity) + "|" + f.Location.Method + ":" + f.Location.PathTemplate
}

func AnalyzeConsistency(runPaths []string, threshold float64) (ConsistencyReport, error) {
	if len(runPaths) < 2 {
		return ConsistencyReport{}, fmt.Errorf("at least 2 run paths required, got %d", len(runPaths))
	}

	type keyInfo struct {
		class    string
		severity string
		summary  string
		count    int
	}
	presence := map[string]*keyInfo{}
	totalFindings := 0

	for _, p := range runPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			return ConsistencyReport{}, fmt.Errorf("reading %s: %w", p, err)
		}
		var findings []model.Finding
		if err := json.Unmarshal(data, &findings); err != nil {
			return ConsistencyReport{}, fmt.Errorf("parsing %s: %w", p, err)
		}
		totalFindings += len(findings)
		seen := map[string]bool{}
		for _, f := range findings {
			k := findingKey(f)
			if seen[k] {
				continue
			}
			seen[k] = true
			if _, ok := presence[k]; !ok {
				presence[k] = &keyInfo{
					class:    string(f.Class),
					severity: string(f.Severity),
					summary:  f.Summary,
				}
			}
			presence[k].count++
		}
	}

	nRuns := len(runPaths)
	var findings []FindingPresence
	stable, flaky, missing := 0, 0, 0

	for k, info := range presence {
		rate := float64(info.count) / float64(nRuns)
		status := "stable"
		if rate >= 1.0 {
			stable++
		} else if rate >= threshold {
			status = "mostly-stable"
			stable++
		} else if rate > 0 {
			status = "flaky"
			flaky++
		} else {
			status = "missing"
			missing++
		}
		findings = append(findings, FindingPresence{
			Key:       k,
			Class:     info.class,
			Severity:  info.severity,
			Summary:   truncate(info.summary, 80),
			RunCount:  info.count,
			TotalRuns: nRuns,
			Rate:      math.Round(rate*1000) / 1000,
			Status:    status,
		})
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Rate != findings[j].Rate {
			return findings[i].Rate > findings[j].Rate
		}
		return findings[i].Key < findings[j].Key
	})

	score := 0.0
	if len(findings) > 0 {
		sum := 0.0
		for _, f := range findings {
			sum += f.Rate
		}
		score = math.Round(sum/float64(len(findings))*1000) / 1000
	}

	return ConsistencyReport{
		TotalRuns:       nRuns,
		TotalFindings:   totalFindings,
		UniqueFindings:  len(findings),
		StableFindings:  stable,
		FlakyFindings:   flaky,
		MissingFindings: missing,
		OverallScore:    score,
		Threshold:       threshold,
		Findings:        findings,
	}, nil
}

func FormatText(r ConsistencyReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Consistency Report (%d runs, %d unique findings)\n\n", r.TotalRuns, r.UniqueFindings)
	fmt.Fprintf(&b, "  Overall score: %.1f%%\n", r.OverallScore*100)
	fmt.Fprintf(&b, "  Stable:        %d\n", r.StableFindings)
	fmt.Fprintf(&b, "  Flaky:         %d\n", r.FlakyFindings)
	fmt.Fprintf(&b, "  Threshold:     %.0f%%\n\n", r.Threshold*100)

	if len(r.Findings) == 0 {
		b.WriteString("  No findings across runs.\n")
		return b.String()
	}

	for _, f := range r.Findings {
		marker := " "
		switch f.Status {
		case "stable", "mostly-stable":
			marker = "+"
		case "flaky":
			marker = "~"
		case "missing":
			marker = "-"
		}
		fmt.Fprintf(&b, "  [%s] %-8s %d/%d (%.0f%%) %s  %s\n",
			marker, f.Severity, f.RunCount, f.TotalRuns,
			f.Rate*100, f.Class, f.Summary)
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
