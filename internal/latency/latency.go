package latency

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ModelLatency struct {
	Agent     string  `json:"agent"`
	Model     string  `json:"model"`
	Provider  string  `json:"provider"`
	LatencyMS int64   `json:"latency_ms"`
	P50MS     float64 `json:"p50_ms"`
	P95MS     float64 `json:"p95_ms"`
	P99MS     float64 `json:"p99_ms"`
	IsOutlier bool    `json:"is_outlier"`
	Grade     string  `json:"grade"`
}

type Report struct {
	RunID      string         `json:"run_id"`
	Models     []ModelLatency `json:"models"`
	Bottleneck string         `json:"bottleneck"`
	Summary    Summary        `json:"summary"`
}

type Summary struct {
	TotalModels int     `json:"total_models"`
	MeanMS      float64 `json:"mean_ms"`
	MedianMS    float64 `json:"median_ms"`
	P95MS       float64 `json:"p95_ms"`
	P99MS       float64 `json:"p99_ms"`
	MinMS       int64   `json:"min_ms"`
	MaxMS       int64   `json:"max_ms"`
	StdDevMS    float64 `json:"std_dev_ms"`
	OutlierPct  float64 `json:"outlier_pct"`
}

func Analyze(rr model.RunReport) Report {
	var latencies []int64
	var models []ModelLatency

	for _, mr := range rr.Models {
		lat := mr.Usage.LatencyMS
		latencies = append(latencies, lat)
		models = append(models, ModelLatency{
			Agent:     mr.Ref.Agent,
			Model:     mr.Ref.Model,
			Provider:  mr.Ref.Provider,
			LatencyMS: lat,
		})
	}

	if len(latencies) == 0 {
		return Report{RunID: rr.RunID, Summary: Summary{}}
	}

	sorted := make([]float64, len(latencies))
	for i, v := range latencies {
		sorted[i] = float64(v)
	}
	sort.Float64s(sorted)

	avg := mean(sorted)
	stdDev := stddev(sorted, avg)

	q1 := percentile(sorted, 25)
	q3 := percentile(sorted, 75)
	iqr := q3 - q1
	upperFence := q3 + 1.5*iqr

	for i := range models {
		models[i].P50MS = percentile(sorted, 50)
		models[i].P95MS = percentile(sorted, 95)
		models[i].P99MS = percentile(sorted, 99)
		models[i].IsOutlier = iqr > 0 && float64(models[i].LatencyMS) > upperFence
		models[i].Grade = gradeLatency(models[i].LatencyMS)
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].LatencyMS > models[j].LatencyMS
	})

	bottleneck := ""
	if len(models) > 0 {
		bottleneck = models[0].Model
	}

	outlierCount := 0
	for _, m := range models {
		if m.IsOutlier {
			outlierCount++
		}
	}

	return Report{
		RunID:      rr.RunID,
		Models:     models,
		Bottleneck: bottleneck,
		Summary: Summary{
			TotalModels: len(models),
			MeanMS:      math.Round(avg*100) / 100,
			MedianMS:    percentile(sorted, 50),
			P95MS:       percentile(sorted, 95),
			P99MS:       percentile(sorted, 99),
			MinMS:       int64(sorted[0]),
			MaxMS:       int64(sorted[len(sorted)-1]),
			StdDevMS:    math.Round(stdDev*100) / 100,
			OutlierPct:  math.Round(float64(outlierCount)/float64(len(models))*10000) / 100,
		},
	}
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func stddev(vals []float64, m float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		d := v - m
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(vals)))
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	idx := p / 100 * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

func gradeLatency(ms int64) string {
	switch {
	case ms <= 500:
		return "A"
	case ms <= 2000:
		return "B"
	case ms <= 5000:
		return "C"
	case ms <= 10000:
		return "D"
	default:
		return "F"
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Model Latency Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")
	b.WriteString(fmt.Sprintf("  Run:        %s\n", r.RunID))
	b.WriteString(fmt.Sprintf("  Models:     %d\n", r.Summary.TotalModels))
	b.WriteString(fmt.Sprintf("  Mean:       %.0f ms\n", r.Summary.MeanMS))
	b.WriteString(fmt.Sprintf("  Median:     %.0f ms\n", r.Summary.MedianMS))
	b.WriteString(fmt.Sprintf("  P95:        %.0f ms\n", r.Summary.P95MS))
	b.WriteString(fmt.Sprintf("  P99:        %.0f ms\n", r.Summary.P99MS))
	b.WriteString(fmt.Sprintf("  Std Dev:    %.0f ms\n", r.Summary.StdDevMS))
	b.WriteString(fmt.Sprintf("  Min:        %d ms\n", r.Summary.MinMS))
	b.WriteString(fmt.Sprintf("  Max:        %d ms\n", r.Summary.MaxMS))
	if r.Bottleneck != "" {
		b.WriteString(fmt.Sprintf("  Bottleneck: %s\n", r.Bottleneck))
	}
	b.WriteByte('\n')

	if len(r.Models) > 0 {
		b.WriteString(fmt.Sprintf("  %-15s %-20s %10s %8s %8s\n", "Agent", "Model", "Latency", "Outlier", "Grade"))
		b.WriteString("  " + strings.Repeat("-", 64) + "\n")
		for _, m := range r.Models {
			outlier := ""
			if m.IsOutlier {
				outlier = "YES"
			}
			b.WriteString(fmt.Sprintf("  %-15s %-20s %8d ms %8s %8s\n",
				m.Agent, m.Model, m.LatencyMS, outlier, m.Grade))
		}
		b.WriteByte('\n')
	}

	return b.String()
}
