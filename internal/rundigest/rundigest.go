package rundigest

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

type Digest struct {
	RunID       string         `json:"run_id"`
	Verdict     string         `json:"verdict"`
	Score       float64        `json:"weighted_score"`
	Models      int            `json:"models"`
	TopModel    string         `json:"top_model,omitempty"`
	TotalCost   float64        `json:"total_cost_usd"`
	Duration    string         `json:"duration"`
	Validated   int            `json:"validated"`
	Refuted     int            `json:"refuted"`
	Theorized   int            `json:"theorized"`
	Refusals    int            `json:"refusals"`
	AvgLatency  float64        `json:"avg_latency_ms"`
	OneLine     string         `json:"one_line"`
	Paragraph   string         `json:"paragraph"`
}

func Summarize(rr model.RunReport) Digest {
	d := Digest{
		RunID:   rr.RunID,
		Verdict: rr.Verdict,
		Score:   rr.WeightedScore,
		Models:  len(rr.Models),
	}

	var totalCost float64
	var latencies []float64
	var totalValidated, totalRefuted, totalTheorized, totalRefusals int

	type modelScore struct {
		name  string
		score float64
	}
	var mScores []modelScore

	for _, m := range rr.Models {
		totalCost += m.Usage.CostUSD
		totalRefusals += m.Refusals

		v := m.Findings[model.StateValidated]
		r := m.Findings[model.StateRefuted]
		t := m.Findings[model.StateTheorized]
		totalValidated += v
		totalRefuted += r
		totalTheorized += t

		if m.P50MS > 0 {
			latencies = append(latencies, float64(m.P50MS))
		}

		quality := float64(v) - float64(r)*0.5
		mScores = append(mScores, modelScore{
			name:  m.Ref.Model,
			score: quality,
		})
	}

	d.TotalCost = math.Round(totalCost*10000) / 10000
	d.Validated = totalValidated
	d.Refuted = totalRefuted
	d.Theorized = totalTheorized
	d.Refusals = totalRefusals

	if len(latencies) > 0 {
		var sum float64
		for _, l := range latencies {
			sum += l
		}
		d.AvgLatency = math.Round(sum / float64(len(latencies)))
	}

	sort.Slice(mScores, func(i, j int) bool {
		return mScores[i].score > mScores[j].score
	})
	if len(mScores) > 0 {
		d.TopModel = mScores[0].name
	}

	if !rr.Started.IsZero() && !rr.Finished.IsZero() {
		d.Duration = formatDuration(rr.Finished.Sub(rr.Started))
	}

	d.OneLine = buildOneLine(d)
	d.Paragraph = buildParagraph(d)

	return d
}

func formatDuration(dur time.Duration) string {
	if dur < 0 {
		return ""
	}
	if dur < time.Minute {
		return fmt.Sprintf("%.0fs", dur.Seconds())
	}
	if dur < time.Hour {
		return fmt.Sprintf("%.1fm", dur.Minutes())
	}
	return fmt.Sprintf("%.1fh", dur.Hours())
}

func buildOneLine(d Digest) string {
	emoji := "?"
	switch strings.ToLower(d.Verdict) {
	case "pass":
		emoji = "PASS"
	case "fail":
		emoji = "FAIL"
	case "error":
		emoji = "ERROR"
	default:
		emoji = strings.ToUpper(d.Verdict)
	}
	parts := []string{
		fmt.Sprintf("[%s]", emoji),
		fmt.Sprintf("score=%.1f%%", d.Score),
		fmt.Sprintf("models=%d", d.Models),
		fmt.Sprintf("v=%d/r=%d/t=%d", d.Validated, d.Refuted, d.Theorized),
		fmt.Sprintf("$%.4f", d.TotalCost),
	}
	if d.Duration != "" {
		parts = append(parts, d.Duration)
	}
	return strings.Join(parts, " ")
}

func buildParagraph(d Digest) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Run %s finished", d.RunID))
	if d.Duration != "" {
		sb.WriteString(fmt.Sprintf(" in %s", d.Duration))
	}
	sb.WriteString(fmt.Sprintf(" with verdict %s (score %.1f%%).", strings.ToUpper(d.Verdict), d.Score))

	sb.WriteString(fmt.Sprintf(" %d model(s) evaluated", d.Models))
	if d.TopModel != "" {
		sb.WriteString(fmt.Sprintf("; top performer: %s", d.TopModel))
	}
	sb.WriteString(".")

	sb.WriteString(fmt.Sprintf(" Findings: %d validated, %d refuted, %d theorized.", d.Validated, d.Refuted, d.Theorized))

	if d.Refusals > 0 {
		sb.WriteString(fmt.Sprintf(" %d refusal(s) observed.", d.Refusals))
	}

	sb.WriteString(fmt.Sprintf(" Total cost: $%.4f.", d.TotalCost))

	if d.AvgLatency > 0 {
		sb.WriteString(fmt.Sprintf(" Avg p50 latency: %.0fms.", d.AvgLatency))
	}

	return sb.String()
}

func FormatText(d Digest) string {
	var sb strings.Builder
	sb.WriteString("Run Digest\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")
	sb.WriteString(fmt.Sprintf("  Run:       %s\n", d.RunID))
	sb.WriteString(fmt.Sprintf("  Verdict:   %s\n", strings.ToUpper(d.Verdict)))
	sb.WriteString(fmt.Sprintf("  Score:     %.1f%%\n", d.Score))
	sb.WriteString(fmt.Sprintf("  Models:    %d\n", d.Models))
	if d.TopModel != "" {
		sb.WriteString(fmt.Sprintf("  Top Model: %s\n", d.TopModel))
	}
	if d.Duration != "" {
		sb.WriteString(fmt.Sprintf("  Duration:  %s\n", d.Duration))
	}
	sb.WriteString(fmt.Sprintf("  Cost:      $%.4f\n", d.TotalCost))
	sb.WriteString(fmt.Sprintf("  Validated: %d\n", d.Validated))
	sb.WriteString(fmt.Sprintf("  Refuted:   %d\n", d.Refuted))
	sb.WriteString(fmt.Sprintf("  Theorized: %d\n", d.Theorized))
	if d.Refusals > 0 {
		sb.WriteString(fmt.Sprintf("  Refusals:  %d\n", d.Refusals))
	}
	if d.AvgLatency > 0 {
		sb.WriteString(fmt.Sprintf("  Avg p50:   %.0fms\n", d.AvgLatency))
	}
	sb.WriteString("\n  One-line: " + d.OneLine + "\n\n")
	sb.WriteString("  " + d.Paragraph + "\n")
	return sb.String()
}
