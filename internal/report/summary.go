package report

import (
	"encoding/json"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/reconcile"
)

// Summary is a machine-readable aggregate of a run's results.
type Summary struct {
	RunID         string         `json:"run_id"`
	Verdict       string         `json:"verdict"`
	BySeverity    map[string]int `json:"by_severity"`
	ByClass       map[string]int `json:"by_class"`
	ByState       map[string]int `json:"by_state"`
	ByModel       map[string]int `json:"by_model"`
	TotalFindings int            `json:"total_findings"`
	Reconciled    int            `json:"reconciled"`
	Mismatched    int            `json:"mismatched"`
	SpentUSD      float64        `json:"spent_usd"`
	BudgetCapUSD  float64        `json:"budget_cap_usd"`
	DurationSec   float64        `json:"duration_sec"`
	Models        []string       `json:"models"`
	Labs          []string       `json:"labs"`
}

// BuildSummary aggregates a RunReport, its findings and optional reconcile
// report into a Summary suitable for CI dashboards and programmatic use.
func BuildSummary(rr model.RunReport, findings []model.Finding, rec *reconcile.Report) Summary {
	s := Summary{
		RunID:        rr.RunID,
		Verdict:      rr.Verdict,
		BySeverity:   map[string]int{},
		ByClass:      map[string]int{},
		ByState:      map[string]int{},
		ByModel:      map[string]int{},
		SpentUSD:     rr.SpentUSD,
		BudgetCapUSD: rr.BudgetCapUSD,
		DurationSec:  rr.Finished.Sub(rr.Started).Seconds(),
		Labs:         rr.Labs,
	}

	modelSet := map[string]bool{}
	for _, m := range rr.Models {
		modelSet[m.Ref.Model] = true
	}
	for m := range modelSet {
		s.Models = append(s.Models, m)
	}

	for _, f := range findings {
		s.TotalFindings++
		s.BySeverity[string(f.Severity)]++
		s.ByClass[string(f.Class)]++
		s.ByState[string(f.State)]++
		s.ByModel[f.Model.Model]++
	}

	if rec != nil {
		s.Reconciled = rec.Checked - len(rec.Mismatches)
		s.Mismatched = len(rec.Mismatches)
	}

	return s
}

// SummaryJSON renders a Summary as indented JSON.
func SummaryJSON(s Summary) (string, error) {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b) + "\n", nil
}
