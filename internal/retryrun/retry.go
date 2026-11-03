package retryrun

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type FailedChecker struct {
	Model    string `json:"model"`
	Provider string `json:"provider"`
	Refusals int    `json:"refusals"`
	Failures int    `json:"failures"`
}

type RetryResult struct {
	RunID          string          `json:"run_id"`
	FailedCheckers []FailedChecker `json:"failed_checkers"`
	TotalFailed    int             `json:"total_failed"`
	TotalSucceeded int             `json:"total_succeeded"`
	MaxRetries     int             `json:"max_retries"`
}

func AnalyzeFailures(rr model.RunReport) RetryResult {
	var failed []FailedChecker
	succeeded := 0

	for _, mr := range rr.Models {
		if mr.Refusals > 0 || mr.Failovers > 0 {
			failed = append(failed, FailedChecker{
				Model:    mr.Ref.Model,
				Provider: mr.Ref.Provider,
				Refusals: mr.Refusals,
				Failures: mr.Failovers,
			})
		} else {
			succeeded++
		}
	}

	return RetryResult{
		RunID:          rr.RunID,
		FailedCheckers: failed,
		TotalFailed:    len(failed),
		TotalSucceeded: succeeded,
	}
}

func FormatText(r RetryResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Retry Analysis: run %s\n\n", r.RunID)
	fmt.Fprintf(&b, "  Succeeded: %d\n", r.TotalSucceeded)
	fmt.Fprintf(&b, "  Failed:    %d\n\n", r.TotalFailed)

	if len(r.FailedCheckers) == 0 {
		b.WriteString("  No failures to retry.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  %-30s %-12s %-10s %s\n", "MODEL", "PROVIDER", "REFUSALS", "FAILOVERS")
	fmt.Fprintf(&b, "  %-30s %-12s %-10s %s\n", "-----", "--------", "--------", "---------")
	for _, fc := range r.FailedCheckers {
		fmt.Fprintf(&b, "  %-30s %-12s %-10d %d\n", fc.Model, fc.Provider, fc.Refusals, fc.Failures)
	}

	if r.MaxRetries > 0 {
		fmt.Fprintf(&b, "\n  Max retries: %d (with exponential backoff)\n", r.MaxRetries)
	}

	return b.String()
}

func FormatJSON(r RetryResult) (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
