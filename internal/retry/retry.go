package retry

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

type FailureKind string

const (
	FailError    FailureKind = "ERROR"
	FailBlocked  FailureKind = "BLOCKED"
	FailTruncate FailureKind = "TRUNCATED"
)

type FailedModel struct {
	Checker     string      `json:"checker"`
	Model       string      `json:"model"`
	FailureKind FailureKind `json:"failure_kind"`
	Refusals    int         `json:"refusals"`
	Failovers   int         `json:"failovers"`
}

type RetryPlan struct {
	RunID        string        `json:"run_id"`
	Verdict      string        `json:"verdict"`
	ExitCode     int           `json:"exit_code"`
	ShouldRetry  bool          `json:"should_retry"`
	Reason       string        `json:"reason"`
	FailedModels []FailedModel `json:"failed_models"`
	MaxRetries   int           `json:"max_retries"`
	Delays       []Duration    `json:"delays"`
}

type Duration struct {
	Attempt int           `json:"attempt"`
	Delay   time.Duration `json:"-"`
	DelayS  string        `json:"delay"`
}

func Analyze(rr model.RunReport, maxRetries int) RetryPlan {
	plan := RetryPlan{
		RunID:      rr.RunID,
		Verdict:    rr.Verdict,
		ExitCode:   rr.ExitCode,
		MaxRetries: maxRetries,
	}

	for _, mr := range rr.Models {
		var kind FailureKind
		switch {
		case mr.Refusals > 0:
			kind = FailBlocked
		case mr.Failovers > 0:
			kind = FailError
		default:
			continue
		}
		plan.FailedModels = append(plan.FailedModels, FailedModel{
			Checker:     mr.Ref.Agent,
			Model:       mr.Ref.Model,
			FailureKind: kind,
			Refusals:    mr.Refusals,
			Failovers:   mr.Failovers,
		})
	}

	if rr.Truncated {
		plan.FailedModels = append(plan.FailedModels, FailedModel{
			FailureKind: FailTruncate,
			Checker:     "(run)",
			Model:       "(all)",
		})
	}

	sort.Slice(plan.FailedModels, func(i, j int) bool {
		return plan.FailedModels[i].Checker < plan.FailedModels[j].Checker
	})

	switch {
	case rr.Verdict == "PASS":
		plan.Reason = "run passed; no retry needed"
	case len(plan.FailedModels) == 0 && !rr.Truncated:
		if rr.Verdict == "ERROR" || rr.ExitCode != 0 {
			plan.ShouldRetry = true
			plan.Reason = "run failed with non-zero exit; transient error likely"
		} else {
			plan.Reason = "no retryable failures detected"
		}
	default:
		plan.ShouldRetry = true
		plan.Reason = fmt.Sprintf("%d model(s) with failures", len(plan.FailedModels))
	}

	if plan.ShouldRetry && maxRetries > 0 {
		plan.Delays = backoffSchedule(maxRetries)
	}

	return plan
}

func backoffSchedule(n int) []Duration {
	ds := make([]Duration, n)
	for i := range n {
		secs := math.Pow(2, float64(i)) * 30
		if secs > 900 {
			secs = 900
		}
		d := time.Duration(secs) * time.Second
		ds[i] = Duration{
			Attempt: i + 1,
			Delay:   d,
			DelayS:  d.String(),
		}
	}
	return ds
}

func FormatText(p RetryPlan) string {
	var b strings.Builder
	b.WriteString("Retry Analysis\n")
	b.WriteString("══════════════════════════════════════════════════════════════\n\n")

	b.WriteString(fmt.Sprintf("  Run:     %s\n", p.RunID))
	b.WriteString(fmt.Sprintf("  Verdict: %s (exit %d)\n", p.Verdict, p.ExitCode))
	b.WriteString(fmt.Sprintf("  Retry:   %v\n", p.ShouldRetry))
	b.WriteString(fmt.Sprintf("  Reason:  %s\n\n", p.Reason))

	if len(p.FailedModels) > 0 {
		b.WriteString(fmt.Sprintf("  %-20s %-25s %-10s %8s %9s\n",
			"Checker", "Model", "Failure", "Refusals", "Failovers"))
		b.WriteString("  ──────────────────────────────────────────────────────────────────────────────\n")
		for _, fm := range p.FailedModels {
			b.WriteString(fmt.Sprintf("  %-20s %-25s %-10s %8d %9d\n",
				fm.Checker, fm.Model, fm.FailureKind, fm.Refusals, fm.Failovers))
		}
		b.WriteByte('\n')
	}

	if len(p.Delays) > 0 {
		b.WriteString("  Backoff Schedule:\n")
		for _, d := range p.Delays {
			b.WriteString(fmt.Sprintf("    Attempt %d: wait %s\n", d.Attempt, d.DelayS))
		}
		b.WriteByte('\n')
	}

	return b.String()
}
