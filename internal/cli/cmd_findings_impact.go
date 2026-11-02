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
	Register("findings-impact", "estimate business impact of findings", runFindingsImpact)
}

type impactArea struct {
	Name     string `json:"name"`
	Count    int    `json:"count"`
	Score    float64 `json:"score"`
}

type impactEntry struct {
	Lab       string   `json:"lab"`
	Areas     []string `json:"impact_areas"`
	Score     float64  `json:"impact_score"`
	ModelHits int      `json:"model_hits"`
}

type impactResult struct {
	TotalFindings int           `json:"total_findings"`
	AreaBreakdown []impactArea  `json:"area_breakdown"`
	TopImpact     []impactEntry `json:"top_impact"`
	AggregateScore float64     `json:"aggregate_score"`
}

func runFindingsImpact(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-impact", stderr)
	inputDir := fs.String("path", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	threshold := fs.Float64("threshold", 0, "minimum impact score to include")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *inputDir == "" {
		fmt.Fprintln(stderr, "assay findings-impact: --path is required")
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*inputDir, runJSON))
	if err != nil {
		return fail(stderr, "findings-impact", err)
	}

	result := analyzeImpact(rr, *threshold)

	switch *format {
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	default:
		fmt.Fprint(stdout, formatImpactText(result))
	}
	return ExitPass
}

var labImpactAreas = map[string][]string{
	"sqli":           {"data_integrity", "confidentiality"},
	"xss":            {"confidentiality", "compliance"},
	"auth_bypass":    {"confidentiality", "compliance", "data_integrity"},
	"ssrf":           {"availability", "confidentiality"},
	"rce":            {"availability", "data_integrity", "confidentiality"},
	"path_traversal": {"confidentiality", "data_integrity"},
	"idor":           {"confidentiality", "compliance"},
	"csrf":           {"data_integrity"},
	"dos":            {"availability"},
	"info_leak":      {"confidentiality"},
}

func classifyLabImpact(lab string) []string {
	lower := strings.ToLower(lab)
	for key, areas := range labImpactAreas {
		if strings.Contains(lower, key) {
			return areas
		}
	}
	return []string{"confidentiality"}
}

func labImpactScore(lab string, modelHits int) float64 {
	areas := classifyLabImpact(lab)
	base := float64(len(areas)) * 2.5
	agreement := float64(modelHits) * 0.5
	return base + agreement
}

func analyzeImpact(rr model.RunReport, threshold float64) impactResult {
	labModels := map[string]int{}
	for _, m := range rr.Models {
		for state, count := range m.Findings {
			if state == "VALIDATED" || state == "THEORIZED" {
				labModels[m.Ref.Model] += count
			}
		}
	}

	areaCounts := map[string]int{}
	areaScores := map[string]float64{}
	var entries []impactEntry

	for _, lab := range rr.Labs {
		hits := 0
		for _, m := range rr.Models {
			total := 0
			for _, c := range m.Findings {
				total += c
			}
			if total > 0 {
				hits++
			}
		}

		areas := classifyLabImpact(lab)
		score := labImpactScore(lab, hits)

		if threshold > 0 && score < threshold {
			continue
		}

		for _, a := range areas {
			areaCounts[a]++
			areaScores[a] += score
		}

		entries = append(entries, impactEntry{
			Lab:       lab,
			Areas:     areas,
			Score:     score,
			ModelHits: hits,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score > entries[j].Score
	})
	if len(entries) > 10 {
		entries = entries[:10]
	}

	var areas []impactArea
	for _, name := range []string{"confidentiality", "data_integrity", "availability", "compliance"} {
		if areaCounts[name] > 0 {
			areas = append(areas, impactArea{Name: name, Count: areaCounts[name], Score: areaScores[name]})
		}
	}

	aggregate := 0.0
	for _, s := range areaScores {
		aggregate += s
	}

	return impactResult{
		TotalFindings:  len(rr.Labs),
		AreaBreakdown:  areas,
		TopImpact:      entries,
		AggregateScore: aggregate,
	}
}

func formatImpactText(r impactResult) string {
	var b strings.Builder
	b.WriteString("Findings Impact Analysis\n")
	b.WriteString("========================================\n")
	fmt.Fprintf(&b, "Total labs: %d\n", r.TotalFindings)
	fmt.Fprintf(&b, "Aggregate impact score: %.1f\n\n", r.AggregateScore)

	if len(r.AreaBreakdown) > 0 {
		b.WriteString("Impact by area:\n")
		for _, a := range r.AreaBreakdown {
			fmt.Fprintf(&b, "  %-20s %3d findings  (score: %.1f)\n", a.Name, a.Count, a.Score)
		}
		b.WriteString("\n")
	}

	if len(r.TopImpact) > 0 {
		b.WriteString("Highest impact findings:\n")
		for i, e := range r.TopImpact {
			fmt.Fprintf(&b, "  %d. %s  score=%.1f  models=%d  [%s]\n",
				i+1, e.Lab, e.Score, e.ModelHits, strings.Join(e.Areas, ", "))
		}
	}
	return b.String()
}
