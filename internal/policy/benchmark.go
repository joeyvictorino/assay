package policy

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

// BenchmarkResult holds throughput measurements for policy evaluation.
type BenchmarkResult struct {
	TotalEvaluations int     `json:"total_evaluations"`
	TotalDurationMS  int64   `json:"total_duration_ms"`
	AvgPerEvalUS     float64 `json:"avg_per_eval_us"`
	MinPerEvalUS     float64 `json:"min_per_eval_us"`
	MaxPerEvalUS     float64 `json:"max_per_eval_us"`
	EvalPerSecond    float64 `json:"eval_per_second"`
	Agents           int     `json:"agents"`
	Tools            int     `json:"tools"`
	Iterations       int     `json:"iterations"`
}

// Benchmark evaluates every agent×tool pair for the given number of iterations
// and returns throughput statistics.
func Benchmark(eng *Engine, manifests []model.ToolManifest, iterations int) BenchmarkResult {
	if iterations < 1 {
		iterations = 1
	}

	p := eng.Policy()
	agents := make([]string, 0, len(p.Agents))
	for name := range p.Agents {
		agents = append(agents, name)
	}
	sort.Strings(agents)

	pairsPerIter := len(agents) * len(manifests)
	if pairsPerIter == 0 {
		return BenchmarkResult{Iterations: iterations, Agents: len(agents), Tools: len(manifests)}
	}

	totalEvals := pairsPerIter * iterations
	durations := make([]time.Duration, 0, totalEvals)

	for i := 0; i < iterations; i++ {
		for _, agent := range agents {
			for _, tool := range manifests {
				req := model.PolicyRequest{
					Agent:           agent,
					Tool:            tool,
					SignatureOK:     true,
					SignatureReason: "BENCHMARK",
				}
				start := time.Now()
				eng.Evaluate(context.Background(), req)
				durations = append(durations, time.Since(start))
			}
		}
	}

	var totalDur time.Duration
	minDur := durations[0]
	maxDur := durations[0]
	for _, d := range durations {
		totalDur += d
		if d < minDur {
			minDur = d
		}
		if d > maxDur {
			maxDur = d
		}
	}

	totalMS := totalDur.Milliseconds()
	avgUS := float64(totalDur.Microseconds()) / float64(totalEvals)
	minUS := float64(minDur.Microseconds())
	maxUS := float64(maxDur.Microseconds())

	var eps float64
	if totalDur > 0 {
		eps = float64(totalEvals) / totalDur.Seconds()
	}

	return BenchmarkResult{
		TotalEvaluations: totalEvals,
		TotalDurationMS:  totalMS,
		AvgPerEvalUS:     math.Round(avgUS*100) / 100,
		MinPerEvalUS:     math.Round(minUS*100) / 100,
		MaxPerEvalUS:     math.Round(maxUS*100) / 100,
		EvalPerSecond:    math.Round(eps*100) / 100,
		Agents:           len(agents),
		Tools:            len(manifests),
		Iterations:       iterations,
	}
}

// FormatBenchmarkText renders the result as human-readable text.
func FormatBenchmarkText(r BenchmarkResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Policy Benchmark\n")
	fmt.Fprintf(&b, "  Agents:      %d\n", r.Agents)
	fmt.Fprintf(&b, "  Tools:       %d\n", r.Tools)
	fmt.Fprintf(&b, "  Iterations:  %d\n", r.Iterations)
	fmt.Fprintf(&b, "  Evaluations: %d\n", r.TotalEvaluations)
	fmt.Fprintf(&b, "  Total:       %d ms\n", r.TotalDurationMS)
	fmt.Fprintf(&b, "  Avg:         %.2f us/eval\n", r.AvgPerEvalUS)
	fmt.Fprintf(&b, "  Min:         %.2f us/eval\n", r.MinPerEvalUS)
	fmt.Fprintf(&b, "  Max:         %.2f us/eval\n", r.MaxPerEvalUS)
	fmt.Fprintf(&b, "  Throughput:  %.2f eval/s\n", r.EvalPerSecond)
	return b.String()
}
