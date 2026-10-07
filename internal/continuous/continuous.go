// Package continuous compares a run against a promoted baseline and turns
// the difference into a verdict and an issue body.
//
// A finding is identified by its dedup key (ADR 0007), so two runs that
// phrase the same issue differently still compare equal. A key is "active"
// when its state is theorized or validated; refuted and declined findings
// are not live claims and never count as new or fixed.
package continuous

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/overlap"
)

// Delta is the difference between a baseline run and the current run.
type Delta struct {
	BaselineRunID string           `json:"baseline_run_id"`
	CurrentRunID  string           `json:"current_run_id"`
	New           []string         `json:"new"`       // active now, absent from baseline
	Fixed         []string         `json:"fixed"`     // active in baseline, absent or refuted now
	Regressed     []string         `json:"regressed"` // refuted in baseline and active now, or severity rose
	CostDeltaUSD  float64          `json:"cost_delta_usd"`
	P95DeltaMS    map[string]int64 `json:"p95_delta_ms"` // model -> current p95 - baseline p95
}

// Empty reports whether the delta carries no finding change.
func (d Delta) Empty() bool {
	return len(d.New) == 0 && len(d.Fixed) == 0 && len(d.Regressed) == 0
}

// KeyOf returns the dedup key used for comparisons.
func KeyOf(f model.Finding) string { return overlap.KeyOf(f) }

type summary struct {
	active   bool
	refuted  bool
	severity model.Severity
}

func summarize(fs []model.Finding) map[string]summary {
	out := map[string]summary{}
	for _, f := range fs {
		k := KeyOf(f)
		s := out[k]
		switch f.State {
		case model.StateTheorized, model.StateValidated:
			s.active = true
		case model.StateRefuted:
			s.refuted = true
		}
		if f.Severity.Rank() > s.severity.Rank() {
			s.severity = f.Severity
		}
		out[k] = s
	}
	return out
}

// Diff computes the delta. Keys in each list are sorted and deduplicated.
// P95DeltaMS covers models present in both runs; a model that appears or
// disappears is visible from the run reports and is not a latency change.
func Diff(baseline, current model.RunReport, baseFindings, curFindings []model.Finding) Delta {
	d := Delta{
		BaselineRunID: baseline.RunID,
		CurrentRunID:  current.RunID,
		New:           []string{},
		Fixed:         []string{},
		Regressed:     []string{},
		CostDeltaUSD:  current.SpentUSD - baseline.SpentUSD,
		P95DeltaMS:    map[string]int64{},
	}
	base := summarize(baseFindings)
	cur := summarize(curFindings)
	for k, c := range cur {
		b, inBase := base[k]
		switch {
		case c.active && !inBase:
			d.New = append(d.New, k)
		case c.active && inBase && !b.active && b.refuted:
			d.Regressed = append(d.Regressed, k)
		case c.active && inBase && b.active && c.severity.Rank() > b.severity.Rank():
			d.Regressed = append(d.Regressed, k)
		}
	}
	for k, b := range base {
		if !b.active {
			continue
		}
		c, inCur := cur[k]
		if !inCur || !c.active {
			d.Fixed = append(d.Fixed, k)
		}
	}
	sort.Strings(d.New)
	sort.Strings(d.Fixed)
	sort.Strings(d.Regressed)

	baseP95 := map[string]int64{}
	for _, m := range baseline.Models {
		baseP95[m.Ref.Model] = m.P95MS
	}
	for _, m := range current.Models {
		if b, ok := baseP95[m.Ref.Model]; ok {
			d.P95DeltaMS[m.Ref.Model] = m.P95MS - b
		}
	}
	return d
}

// Verdict returns 1 when any new or regressed key has severity at or above
// failOn, else 0. sevOf maps a key to its severity; a nil sevOf treats
// every key as model.SevInfo. An empty failOn fails on any new or
// regressed key, because every severity ranks at or above it.
func Verdict(d Delta, failOn model.Severity, sevOf func(key string) model.Severity) int {
	if sevOf == nil {
		sevOf = func(string) model.Severity { return model.SevInfo }
	}
	for _, list := range [][]string{d.New, d.Regressed} {
		for _, k := range list {
			if sevOf(k).Rank() >= failOn.Rank() {
				return 1
			}
		}
	}
	return 0
}

// SeverityIndex builds a sevOf function from finding lists; later lists
// override earlier ones for the same key, and the highest severity wins
// within a list.
func SeverityIndex(lists ...[]model.Finding) func(key string) model.Severity {
	idx := map[string]model.Severity{}
	for _, fs := range lists {
		seen := map[string]model.Severity{}
		for _, f := range fs {
			k := KeyOf(f)
			if f.Severity.Rank() >= seen[k].Rank() {
				seen[k] = f.Severity
			}
		}
		for k, s := range seen {
			idx[k] = s
		}
	}
	return func(key string) model.Severity { return idx[key] }
}

// Markdown renders d as a GitHub issue body.
func Markdown(d Delta) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	w("## Continuous validation drift\n\n")
	w("Baseline `%s` compared with run `%s`.\n\n", dash(d.BaselineRunID), dash(d.CurrentRunID))
	w("| Change | Count |\n| --- | ---: |\n")
	w("| New | %d |\n| Fixed | %d |\n| Regressed | %d |\n\n", len(d.New), len(d.Fixed), len(d.Regressed))
	section := func(title string, keys []string) {
		w("### %s\n\n", title)
		if len(keys) == 0 {
			w("None.\n\n")
			return
		}
		for _, k := range keys {
			w("- `%s`\n", k)
		}
		w("\n")
	}
	section("New", d.New)
	section("Regressed", d.Regressed)
	section("Fixed", d.Fixed)
	w("### Cost and latency\n\n")
	w("- Spend delta: %+.4f USD\n", d.CostDeltaUSD)
	if len(d.P95DeltaMS) == 0 {
		w("- p95 latency: no model present in both runs\n")
	} else {
		models := make([]string, 0, len(d.P95DeltaMS))
		for m := range d.P95DeltaMS {
			models = append(models, m)
		}
		sort.Strings(models)
		for _, m := range models {
			w("- p95 latency `%s`: %+d ms\n", m, d.P95DeltaMS[m])
		}
	}
	w("\nKeys are dedup keys (ADR 0007). Regenerate with `assay continuous diff`.\n")
	return b.String()
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
