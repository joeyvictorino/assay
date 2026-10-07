// Package report renders a run into human-readable Markdown and into the
// row format the static dashboard under docs/dashboard reads.
//
// Both renderers are pure functions of their inputs: the same RunReport
// produces the same bytes, so committed reports can be regenerated and
// compared in CI.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/reconcile"
)

// NoGroundTruth is the sentence printed under Evaluation when a run has no
// precision table.
const NoGroundTruth = "No ground truth for this lab; recall only against the reference list."

// states is the fixed column order for state tables.
var states = []model.FindingState{model.StateValidated, model.StateTheorized, model.StateRefuted, model.StateDeclined}

// Markdown renders the run report. rec may be nil, in which case the
// Reconciliation section falls back to the reason counts in rr. Sections
// appear in this fixed order: Scope statement, Corpus, Findings, Overlap,
// Evaluation, Reconciliation, Policy decisions, Evidence model, Limitations,
// Runtime.
func Markdown(rr model.RunReport, findings []model.Finding, rec *reconcile.Report) string {
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	w("# assay run %s\n\n", orDash(rr.RunID))
	if rr.Mode == "degraded" {
		w("> Mode: **degraded**. One or more configured providers had no key; their models were dropped before the run started.\n\n")
	}

	// 1. Scope statement.
	w("## Scope statement\n\n")
	w("Every request in this run passed the signed scope gate (decision `%s`, reason `%s`). ", orDash(string(rr.Scope.Effect)), orDash(rr.Scope.Reason))
	w("Targets were the authorized lab services listed under Corpus and nothing else. ")
	w("Findings start `theorized`; only a deterministic checker in an authorized lab can mark one `validated` or `refuted`. ")
	w("No transcript is retained; the audit log holds digests and bounded metadata only.\n\n")

	// 2. Corpus.
	w("## Corpus\n\n")
	w("- Run id: `%s`\n", orDash(rr.RunID))
	w("- Git SHA: `%s`\n", orDash(rr.GitSHA))
	w("- CI run: %s\n", linkOrDash(rr.CIRunURL))
	w("- Mode: %s\n", orDash(rr.Mode))
	w("- Labs: %s\n", listOrDash(rr.Labs))
	w("- Models: %s\n\n", listOrDash(modelNames(rr)))

	// 3. Findings by state and model.
	w("## Findings\n\n")
	if len(rr.Models) == 0 && len(findings) == 0 {
		w("No findings were recorded.\n\n")
	} else {
		w("| Model | Validated | Theorized | Refuted | Declined | Total |\n")
		w("| --- | ---: | ---: | ---: | ---: | ---: |\n")
		counts := countByModel(rr, findings)
		names := sortedKeys(counts)
		for _, name := range names {
			c := counts[name]
			total := 0
			for _, s := range states {
				total += c[s]
			}
			w("| %s | %d | %d | %d | %d | %d |\n", name, c[model.StateValidated], c[model.StateTheorized], c[model.StateRefuted], c[model.StateDeclined], total)
		}
		w("\n")
		if n := len(findings); n > 0 {
			w("%d finding record(s) in `findings.redacted.json`; %d distinct dedup key(s).\n\n", n, distinctKeys(findings))
		}
	}

	// 4. Overlap.
	w("## Overlap\n\n")
	if rr.Overlap == nil || len(rr.Overlap.Models) == 0 {
		w("No overlap matrix (fewer than one model with findings).\n\n")
	} else {
		o := rr.Overlap
		w("Union %d, intersection %d across %d model(s).\n\n", o.Union, o.Intersection, len(o.Models))
		w("| A | B | Both | Only A | Only B | Jaccard |\n")
		w("| --- | --- | ---: | ---: | ---: | ---: |\n")
		for i := range o.Models {
			for j := i + 1; j < len(o.Models); j++ {
				if i < len(o.Pairwise) && j < len(o.Pairwise[i]) {
					c := o.Pairwise[i][j]
					w("| %s | %s | %d | %d | %d | %.3f |\n", o.Models[i], o.Models[j], c.Both, c.OnlyA, c.OnlyB, c.Jaccard)
				}
			}
		}
		if len(o.Models) < 2 {
			w("| %s | - | - | - | - | - |\n", o.Models[0])
		}
		w("\n**Unique per model**\n\n| Model | Unique keys |\n| --- | ---: |\n")
		for _, m := range o.Models {
			w("| %s | %d |\n", m, len(o.UniquePer[m]))
		}
		w("\n")
	}

	// 5. Evaluation.
	w("## Evaluation\n\n")
	if len(rr.Precision) == 0 {
		w("%s\n\n", NoGroundTruth)
	} else {
		w("| Model | TP | FP | FN | Precision | Recall | F1 |\n")
		w("| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
		for _, name := range sortedKeys(rr.Precision) {
			p := rr.Precision[name]
			w("| %s | %d | %d | %d | %.3f | %.3f | %.3f |\n", name, p.TP, p.FP, p.FN, p.Precision, p.Recall, p.F1)
		}
		w("\nGround truth is consulted after detection only; it never changes a finding.\n\n")
	}

	// 6. Reconciliation.
	w("## Reconciliation\n\n")
	switch {
	case rec != nil:
		w("Checked %d tool call id(s); %d mismatch(es).\n\n", rec.Checked, len(rec.Mismatches))
		if len(rec.ReasonCounts) > 0 {
			w("| Reason | Count |\n| --- | ---: |\n")
			for _, r := range sortedKeys(rec.ReasonCounts) {
				w("| %s | %d |\n", r, rec.ReasonCounts[r])
			}
			w("\n")
		}
		if len(rec.Mismatches) > 0 {
			w("| Tool call id | Reasons | Claimed | Actual | Transcript seqs | Control seqs | Confidence |\n")
			w("| --- | --- | --- | --- | --- | --- | --- |\n")
			for _, m := range rec.Mismatches {
				w("| `%s` | %s | %s | %s | %s | %s | %s |\n", m.ToolCallID, strings.Join(m.Reasons, ", "), orDash(m.Claimed), orDash(m.Actual), seqList(m.TranscriptSeqs), seqList(m.ControlSeqs), m.Confidence)
			}
			w("\n")
		}
		w("Findings whose tool calls appear above were downgraded to `theorized`.\n\n")
	case len(rr.Reconcile) > 0:
		w("| Reason | Count |\n| --- | ---: |\n")
		for _, r := range sortedKeys(rr.Reconcile) {
			w("| %s | %d |\n", r, rr.Reconcile[r])
		}
		w("\n")
	default:
		w("No reconciliation mismatches recorded.\n\n")
	}

	// 7. Policy decisions.
	w("## Policy decisions\n\n")
	if len(rr.PolicyDenies) == 0 {
		w("No policy denials recorded.\n\n")
	} else {
		w("| Deny reason | Count |\n| --- | ---: |\n")
		for _, r := range sortedKeys(rr.PolicyDenies) {
			w("| `%s` | %d |\n", r, rr.PolicyDenies[r])
		}
		w("\n")
	}

	// 8. Evidence model.
	w("## Evidence model\n\n")
	w("`model call -> claimed tool call -> policy decision -> executed tool call -> gate decision -> HTTP exchange digest -> finding -> checker evidence`\n\n")
	w("- Audit head hash: `%s`\n", orDash(rr.AuditHead))
	w("- Audit record count: %d\n", rr.AuditCount)
	// LEAD: model.RunReport has no audit key id field. audit.Summary.KeyID
	// holds it; adding `AuditKeyID string` to RunReport would let this line
	// print the real value instead of pointing at the log header.
	w("- Audit key id: see the audit log header (not carried in run.json)\n")
	w("- Transcript retention: none (zero data retention sink)\n\n")

	// 9. Limitations.
	w("## Limitations\n\n")
	for _, l := range limitations(rr) {
		w("- %s\n", l)
	}
	w("\n")

	// 10. Runtime.
	w("## Runtime\n\n")
	w("- Started: %s\n", stamp(rr.Started))
	w("- Finished: %s\n", stamp(rr.Finished))
	if !rr.Started.IsZero() && !rr.Finished.IsZero() {
		w("- Duration: %s\n", rr.Finished.Sub(rr.Started).Round(time.Second))
	}
	w("- Spent: %.4f USD of a %.2f USD cap\n", rr.SpentUSD, rr.BudgetCapUSD)
	if len(rr.Models) > 0 {
		w("\n| Model | Cost USD | p50 ms | p95 ms | Refusals | Failovers |\n")
		w("| --- | ---: | ---: | ---: | ---: | ---: |\n")
		for _, m := range sortedModels(rr) {
			w("| %s | %.4f | %d | %d | %d | %d |\n", m.Ref.Model, m.Usage.CostUSD, m.P50MS, m.P95MS, m.Refusals, m.Failovers)
		}
	}
	w("\n- Truncated: %t\n", rr.Truncated)
	w("- Verdict: %s (exit %d)\n", orDash(rr.Verdict), rr.ExitCode)
	return b.String()
}

// DashboardRow is one line of docs/dashboard/data/index.json.
type DashboardRow struct {
	RunID               string           `json:"run_id"`
	Date                string           `json:"date"`
	CIRunURL            string           `json:"ci_run_url"`
	Models              []DashboardModel `json:"models"`
	OverlapUnion        int              `json:"overlap_union"`
	OverlapIntersection int              `json:"overlap_intersection"`
	Verdict             string           `json:"verdict"`
}

// DashboardModel is the per-model part of a DashboardRow.
type DashboardModel struct {
	Model             string  `json:"model"`
	FindingsValidated int     `json:"findings_validated"`
	FindingsTheorized int     `json:"findings_theorized"`
	Refusals          int     `json:"refusals"`
	CostUSD           float64 `json:"cost_usd"`
	P50MS             int64   `json:"p50_ms"`
	P95MS             int64   `json:"p95_ms"`
}

// Rows converts run reports to dashboard rows sorted by date then run id.
func Rows(runs []model.RunReport) []DashboardRow {
	rows := make([]DashboardRow, 0, len(runs))
	for _, rr := range runs {
		row := DashboardRow{
			RunID:    rr.RunID,
			Date:     dateOf(rr.Started),
			CIRunURL: rr.CIRunURL,
			Models:   []DashboardModel{},
			Verdict:  rr.Verdict,
		}
		for _, m := range sortedModels(rr) {
			row.Models = append(row.Models, DashboardModel{
				Model:             m.Ref.Model,
				FindingsValidated: m.Findings[model.StateValidated],
				FindingsTheorized: m.Findings[model.StateTheorized],
				Refusals:          m.Refusals,
				CostUSD:           m.Usage.CostUSD,
				P50MS:             m.P50MS,
				P95MS:             m.P95MS,
			})
		}
		if rr.Overlap != nil {
			row.OverlapUnion = rr.Overlap.Union
			row.OverlapIntersection = rr.Overlap.Intersection
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date < rows[j].Date
		}
		return rows[i].RunID < rows[j].RunID
	})
	return rows
}

// DashboardData encodes Rows(runs) as indented JSON with a trailing
// newline. An empty input yields "[]\n", the committed placeholder.
func DashboardData(runs []model.RunReport) []byte {
	rows := Rows(runs)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(rows); err != nil {
		// Only unencodable floats can fail here; fall back to the empty list
		// so the dashboard still loads.
		return []byte("[]\n")
	}
	return buf.Bytes()
}

func limitations(rr model.RunReport) []string {
	out := []string{
		"Labs are deliberately vulnerable services; numbers describe recovery on known targets, not expected accuracy on production systems.",
		"Dedup keys are lexical (class, method, templated path, parameter); semantically identical findings at different paths stay distinct.",
		"Classes without a non-destructive checker remain `theorized` and are never counted as validated.",
		"Refusals are recorded and never retried; a model that declines a lab contributes zero findings for it by design.",
		"Provider-side data handling is governed by each provider's agreement and is not measured or claimed here.",
	}
	if rr.Mode == "degraded" {
		out = append(out, "This run was degraded: at least one configured model was dropped for a missing key, so cross-model comparisons are partial.")
	}
	if rr.Truncated {
		out = append(out, "The run hit a budget or latency cap and was truncated; counts are lower bounds.")
	}
	if len(rr.Precision) == 0 {
		out = append(out, "No ground truth was available; precision cannot be stated for this run.")
	}
	return out
}

func countByModel(rr model.RunReport, findings []model.Finding) map[string]map[model.FindingState]int {
	counts := map[string]map[model.FindingState]int{}
	for _, m := range rr.Models {
		c := map[model.FindingState]int{}
		for s, n := range m.Findings {
			c[s] = n
		}
		counts[m.Ref.Model] = c
	}
	if len(counts) > 0 {
		return counts
	}
	for _, f := range findings {
		name := f.Model.Model
		if name == "" {
			name = "(unattributed)"
		}
		if counts[name] == nil {
			counts[name] = map[model.FindingState]int{}
		}
		counts[name][f.State]++
	}
	return counts
}

func distinctKeys(findings []model.Finding) int {
	seen := map[string]bool{}
	for _, f := range findings {
		k := f.DedupKey
		if k == "" {
			k = f.ID
		}
		seen[k] = true
	}
	return len(seen)
}

func modelNames(rr model.RunReport) []string {
	names := make([]string, 0, len(rr.Models))
	for _, m := range sortedModels(rr) {
		names = append(names, m.Ref.Model)
	}
	return names
}

func sortedModels(rr model.RunReport) []model.ModelResult {
	ms := append([]model.ModelResult(nil), rr.Models...)
	sort.SliceStable(ms, func(i, j int) bool {
		if ms[i].Ref.Model != ms[j].Ref.Model {
			return ms[i].Ref.Model < ms[j].Ref.Model
		}
		return ms[i].Ref.Provider < ms[j].Ref.Provider
	})
	return ms
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func seqList(seqs []uint64) string {
	if len(seqs) == 0 {
		return "-"
	}
	parts := make([]string, len(seqs))
	for i, s := range seqs {
		parts[i] = fmt.Sprint(s)
	}
	return strings.Join(parts, " ")
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func linkOrDash(u string) string {
	if strings.TrimSpace(u) == "" {
		return "-"
	}
	return "<" + u + ">"
}

func listOrDash(items []string) string {
	if len(items) == 0 {
		return "-"
	}
	quoted := make([]string, len(items))
	for i, it := range items {
		quoted[i] = "`" + it + "`"
	}
	return strings.Join(quoted, ", ")
}

func stamp(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.UTC().Format(time.RFC3339)
}

func dateOf(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}
