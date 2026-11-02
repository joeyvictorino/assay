package checkeragree

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type CheckerPair struct {
	CheckerA    string  `json:"checker_a"`
	CheckerB    string  `json:"checker_b"`
	Agreements  int     `json:"agreements"`
	Disagreements int   `json:"disagreements"`
	Total       int     `json:"total"`
	AgreeRate   float64 `json:"agree_rate"`
	KappaScore  float64 `json:"kappa_score"`
}

type CheckerStats struct {
	Checker     string  `json:"checker"`
	Validated   int     `json:"validated"`
	Refuted     int     `json:"refuted"`
	Total       int     `json:"total"`
	ValidRate   float64 `json:"valid_rate"`
}

type Report struct {
	TotalFindings  int           `json:"total_findings"`
	CheckerCount   int           `json:"checker_count"`
	OverallKappa   float64       `json:"overall_kappa"`
	Pairs          []CheckerPair `json:"pairs"`
	Checkers       []CheckerStats `json:"checkers"`
	Recommendation string        `json:"recommendation"`
}

func cohensKappa(agree, disagree int) float64 {
	total := agree + disagree
	if total == 0 {
		return 0
	}
	po := float64(agree) / float64(total)
	pe := 0.5
	if po == pe {
		return 0
	}
	k := (po - pe) / (1 - pe)
	return math.Round(k*1000) / 1000
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{Recommendation: "No findings to analyze"}
	}

	type findingKey struct {
		dedupKey string
		checker  string
	}

	checkerResults := make(map[string]map[string]model.FindingState)
	checkerTotals := make(map[string]*struct{ valid, refuted, total int })

	for _, f := range findings {
		if f.Validation == nil {
			continue
		}
		checker := f.Validation.Checker
		if checker == "" {
			continue
		}
		state := f.Validation.State
		if state != model.StateValidated && state != model.StateRefuted {
			continue
		}

		key := f.DedupKey
		if key == "" {
			key = f.ID
		}

		if checkerResults[checker] == nil {
			checkerResults[checker] = make(map[string]model.FindingState)
		}
		checkerResults[checker][key] = state

		ct, ok := checkerTotals[checker]
		if !ok {
			ct = &struct{ valid, refuted, total int }{}
			checkerTotals[checker] = ct
		}
		ct.total++
		if state == model.StateValidated {
			ct.valid++
		} else {
			ct.refuted++
		}
	}

	var checkers []CheckerStats
	for chk, ct := range checkerTotals {
		vr := 0.0
		if ct.total > 0 {
			vr = math.Round(float64(ct.valid)/float64(ct.total)*1000) / 1000
		}
		checkers = append(checkers, CheckerStats{
			Checker:   chk,
			Validated: ct.valid,
			Refuted:   ct.refuted,
			Total:     ct.total,
			ValidRate: vr,
		})
	}
	sort.Slice(checkers, func(i, j int) bool {
		return checkers[i].Checker < checkers[j].Checker
	})

	checkerNames := make([]string, 0, len(checkerResults))
	for n := range checkerResults {
		checkerNames = append(checkerNames, n)
	}
	sort.Strings(checkerNames)

	var pairs []CheckerPair
	totalAgree := 0
	totalDisagree := 0

	for i := 0; i < len(checkerNames); i++ {
		for j := i + 1; j < len(checkerNames); j++ {
			a := checkerNames[i]
			b := checkerNames[j]
			agree := 0
			disagree := 0
			for key, stateA := range checkerResults[a] {
				if stateB, ok := checkerResults[b][key]; ok {
					if stateA == stateB {
						agree++
					} else {
						disagree++
					}
				}
			}
			if agree+disagree == 0 {
				continue
			}
			total := agree + disagree
			ar := math.Round(float64(agree)/float64(total)*1000) / 1000
			kappa := cohensKappa(agree, disagree)
			pairs = append(pairs, CheckerPair{
				CheckerA:      a,
				CheckerB:      b,
				Agreements:    agree,
				Disagreements: disagree,
				Total:         total,
				AgreeRate:     ar,
				KappaScore:    kappa,
			})
			totalAgree += agree
			totalDisagree += disagree
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].KappaScore > pairs[j].KappaScore
	})

	overallKappa := cohensKappa(totalAgree, totalDisagree)

	rec := fmt.Sprintf("%d checkers, overall kappa=%.3f", len(checkerNames), overallKappa)
	if overallKappa < 0.4 {
		rec += " (poor agreement — review checker calibration)"
	} else if overallKappa < 0.6 {
		rec += " (moderate agreement)"
	} else {
		rec += " (good agreement)"
	}

	return Report{
		TotalFindings:  len(findings),
		CheckerCount:   len(checkerNames),
		OverallKappa:   overallKappa,
		Pairs:          pairs,
		Checkers:       checkers,
		Recommendation: rec,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Inter-Checker Agreement\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	fmt.Fprintf(&b, "  Total findings:  %d\n", r.TotalFindings)
	fmt.Fprintf(&b, "  Checkers:        %d\n", r.CheckerCount)
	fmt.Fprintf(&b, "  Overall kappa:   %.3f\n", r.OverallKappa)
	fmt.Fprintf(&b, "  %s\n\n", r.Recommendation)

	if len(r.Checkers) > 0 {
		b.WriteString("  Checker Statistics:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, cs := range r.Checkers {
			fmt.Fprintf(&b, "    %-20s validated=%d refuted=%d rate=%.1f%%\n",
				cs.Checker, cs.Validated, cs.Refuted, cs.ValidRate*100)
		}
		b.WriteString("\n")
	}

	if len(r.Pairs) > 0 {
		b.WriteString("  Pairwise Agreement:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, p := range r.Pairs {
			fmt.Fprintf(&b, "    %s vs %s: %d/%d agree (%.1f%%), kappa=%.3f\n",
				p.CheckerA, p.CheckerB, p.Agreements, p.Total,
				p.AgreeRate*100, p.KappaScore)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
