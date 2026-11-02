package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("model-compare", "compare model performance across runs", runModelCompare)
}

type modelCompareEntry struct {
	Model         string  `json:"model"`
	RunCount      int     `json:"run_count"`
	TotalFindings int     `json:"total_findings"`
	AvgFindings   float64 `json:"avg_findings"`
	Refusals      int     `json:"refusals"`
	AvgP50MS      int64   `json:"avg_p50_ms"`
}

type modelOverlap struct {
	ModelA    string  `json:"model_a"`
	ModelB    string  `json:"model_b"`
	Shared    int     `json:"shared_findings"`
	OnlyA     int     `json:"only_a"`
	OnlyB     int     `json:"only_b"`
	Agreement float64 `json:"agreement_pct"`
}

type modelCompareResult struct {
	RunsAnalyzed int                 `json:"runs_analyzed"`
	Models       []modelCompareEntry `json:"models"`
	Overlaps     []modelOverlap      `json:"overlaps,omitempty"`
}

func runModelCompare(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-compare", stderr)
	runsStr := fs.String("runs", "", "comma-separated run directory paths (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runsStr == "" {
		fmt.Fprintln(stderr, "assay model-compare: --runs is required")
		return ExitError
	}

	dirs := strings.Split(*runsStr, ",")
	var reports []model.RunReport
	for _, d := range dirs {
		d = strings.TrimSpace(d)
		rr, err := readRunReport(filepath.Join(d, runJSON))
		if err != nil {
			fmt.Fprintf(stderr, "assay model-compare: skipping %s: %v\n", d, err)
			continue
		}
		reports = append(reports, rr)
	}

	if len(reports) == 0 {
		fmt.Fprintln(stderr, "assay model-compare: no valid runs loaded")
		return ExitError
	}

	result := compareModels(reports)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	default:
		fmt.Fprint(stdout, formatModelCompareText(result))
	}
	return ExitPass
}

func compareModels(reports []model.RunReport) modelCompareResult {
	type modelAgg struct {
		findings int
		refusals int
		p50      int64
		count    int
	}
	agg := map[string]*modelAgg{}

	for _, rr := range reports {
		for _, m := range rr.Models {
			name := m.Ref.Model
			if agg[name] == nil {
				agg[name] = &modelAgg{}
			}
			total := 0
			for _, c := range m.Findings {
				total += c
			}
			agg[name].findings += total
			agg[name].refusals += m.Refusals
			agg[name].p50 += m.P50MS
			agg[name].count++
		}
	}

	var entries []modelCompareEntry
	for name, a := range agg {
		avg := float64(a.findings) / float64(a.count)
		avgP50 := a.p50 / int64(a.count)
		entries = append(entries, modelCompareEntry{
			Model:         name,
			RunCount:      a.count,
			TotalFindings: a.findings,
			AvgFindings:   avg,
			Refusals:      a.refusals,
			AvgP50MS:      avgP50,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].AvgFindings > entries[j].AvgFindings
	})

	var overlaps []modelOverlap
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Model
	}
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			ov := computeOverlap(reports, names[i], names[j])
			overlaps = append(overlaps, ov)
		}
	}

	return modelCompareResult{
		RunsAnalyzed: len(reports),
		Models:       entries,
		Overlaps:     overlaps,
	}
}

func computeOverlap(reports []model.RunReport, a, b string) modelOverlap {
	totalA := 0
	totalB := 0
	for _, rr := range reports {
		for _, m := range rr.Models {
			total := 0
			for _, c := range m.Findings {
				total += c
			}
			if m.Ref.Model == a {
				totalA += total
			}
			if m.Ref.Model == b {
				totalB += total
			}
		}
	}

	shared := min(totalA, totalB)
	onlyA := totalA - shared
	onlyB := totalB - shared

	agreement := 0.0
	total := totalA + totalB
	if total > 0 {
		agreement = float64(shared*2) / float64(total) * 100
	}

	return modelOverlap{
		ModelA:    a,
		ModelB:    b,
		Shared:    shared,
		OnlyA:     onlyA,
		OnlyB:     onlyB,
		Agreement: agreement,
	}
}

func formatModelCompareText(r modelCompareResult) string {
	var b strings.Builder
	b.WriteString("Model Performance Comparison\n")
	b.WriteString("========================================\n")
	fmt.Fprintf(&b, "Runs analyzed: %d\n\n", r.RunsAnalyzed)

	if len(r.Models) > 0 {
		b.WriteString("Model summary:\n")
		fmt.Fprintf(&b, "  %-25s %6s %8s %8s %8s\n", "MODEL", "RUNS", "TOTAL", "AVG", "P50ms")
		for _, m := range r.Models {
			fmt.Fprintf(&b, "  %-25s %6d %8d %8.1f %8d\n",
				m.Model, m.RunCount, m.TotalFindings, m.AvgFindings, m.AvgP50MS)
		}
		b.WriteString("\n")
	}

	if len(r.Overlaps) > 0 {
		b.WriteString("Model agreement:\n")
		for _, o := range r.Overlaps {
			fmt.Fprintf(&b, "  %s vs %s: %.1f%% agreement (shared=%d, only-%s=%d, only-%s=%d)\n",
				o.ModelA, o.ModelB, o.Agreement,
				o.Shared, o.ModelA, o.OnlyA, o.ModelB, o.OnlyB)
		}
	}
	return b.String()
}
