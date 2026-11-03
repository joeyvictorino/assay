package responselatency

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

type LatencyDetail struct {
	MeanMS       float64 `json:"mean_ms"`
	MedianMS     float64 `json:"median_ms"`
	P90MS        float64 `json:"p90_ms"`
	P95MS        float64 `json:"p95_ms"`
	P99MS        float64 `json:"p99_ms"`
	MinMS        float64 `json:"min_ms"`
	MaxMS        float64 `json:"max_ms"`
	StdDevMS     float64 `json:"stddev_ms"`
}

type SlowPattern struct {
	Lab      string  `json:"lab"`
	LatencyMS float64 `json:"latency_ms"`
	Summary  string  `json:"summary"`
}

type ModelLatency struct {
	Model         string        `json:"model"`
	LatencyScore  float64       `json:"latency_score"`
	Grade         string        `json:"grade"`
	Detail        LatencyDetail `json:"detail"`
	SampleCount   int           `json:"sample_count"`
	LabCount      int           `json:"lab_count"`
	TimeoutCount  int           `json:"timeout_count"`
	SlowCount     int           `json:"slow_count"`
	SlowPatterns  []SlowPattern `json:"slow_patterns,omitempty"`
}

type Report struct {
	Models       []ModelLatency `json:"models"`
	Fastest      string         `json:"fastest"`
	Slowest      string         `json:"slowest"`
	MeanScore    float64        `json:"mean_score"`
	TotalTimeout int            `json:"total_timeouts"`
}

const slowThresholdMS = 5000.0
const timeoutThresholdMS = 30000.0

func latencyFromFinding(f model.Finding) float64 {
	if f.FirstSeen.IsZero() || f.LastSeen.IsZero() {
		return 0
	}
	d := f.LastSeen.Sub(f.FirstSeen)
	if d < 0 {
		return 0
	}
	return float64(d) / float64(time.Millisecond)
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100.0 * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

func stddev(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	var sum float64
	for _, v := range values {
		d := v - mean
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(values)))
}

func scoreToGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

func computeScore(detail LatencyDetail, timeoutRatio, slowRatio float64) float64 {
	medianScore := 100.0
	if detail.MedianMS > 0 {
		medianScore = math.Max(0, 100-detail.MedianMS/100)
	}

	p95Score := 100.0
	if detail.P95MS > 0 {
		p95Score = math.Max(0, 100-detail.P95MS/200)
	}

	consistencyScore := 100.0
	if detail.MeanMS > 0 {
		cv := detail.StdDevMS / detail.MeanMS
		consistencyScore = math.Max(0, 100-cv*100)
	}

	timeoutPenalty := timeoutRatio * 100
	slowPenalty := slowRatio * 50

	score := medianScore*0.35 + p95Score*0.25 + consistencyScore*0.2 + (100-timeoutPenalty)*0.1 + (100-slowPenalty)*0.1
	return math.Max(0, math.Min(100, score))
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	type entry struct {
		lab     string
		latency float64
		summary string
	}
	modelEntries := map[string][]entry{}

	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		lat := latencyFromFinding(f)
		modelEntries[mn] = append(modelEntries[mn], entry{
			lab:     f.Lab,
			latency: lat,
			summary: f.Summary,
		})
	}

	var models []ModelLatency
	totalTimeouts := 0

	for mn, entries := range modelEntries {
		var latencies []float64
		labs := map[string]bool{}
		timeouts := 0
		slow := 0
		var slowPatterns []SlowPattern

		for _, e := range entries {
			labs[e.lab] = true
			if e.latency <= 0 {
				continue
			}
			latencies = append(latencies, e.latency)
			if e.latency >= timeoutThresholdMS {
				timeouts++
			}
			if e.latency >= slowThresholdMS {
				slow++
				if len(slowPatterns) < 5 {
					slowPatterns = append(slowPatterns, SlowPattern{
						Lab:       e.lab,
						LatencyMS: math.Round(e.latency*10) / 10,
						Summary:   truncate(e.summary, 80),
					})
				}
			}
		}

		totalTimeouts += timeouts

		if len(latencies) == 0 {
			models = append(models, ModelLatency{
				Model:        mn,
				LatencyScore: 50,
				Grade:        "N/A",
				SampleCount:  len(entries),
				LabCount:     len(labs),
			})
			continue
		}

		sort.Float64s(latencies)

		var sum float64
		for _, l := range latencies {
			sum += l
		}
		mean := sum / float64(len(latencies))

		detail := LatencyDetail{
			MeanMS:   math.Round(mean*10) / 10,
			MedianMS: math.Round(percentile(latencies, 50)*10) / 10,
			P90MS:    math.Round(percentile(latencies, 90)*10) / 10,
			P95MS:    math.Round(percentile(latencies, 95)*10) / 10,
			P99MS:    math.Round(percentile(latencies, 99)*10) / 10,
			MinMS:    math.Round(latencies[0]*10) / 10,
			MaxMS:    math.Round(latencies[len(latencies)-1]*10) / 10,
			StdDevMS: math.Round(stddev(latencies, mean)*10) / 10,
		}

		n := float64(len(latencies))
		timeoutRatio := float64(timeouts) / n
		slowRatio := float64(slow) / n

		score := computeScore(detail, timeoutRatio, slowRatio)

		sort.Slice(slowPatterns, func(i, j int) bool {
			return slowPatterns[i].LatencyMS > slowPatterns[j].LatencyMS
		})

		models = append(models, ModelLatency{
			Model:        mn,
			LatencyScore: math.Round(score*10) / 10,
			Grade:        scoreToGrade(score),
			Detail:       detail,
			SampleCount:  len(entries),
			LabCount:     len(labs),
			TimeoutCount: timeouts,
			SlowCount:    slow,
			SlowPatterns: slowPatterns,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].LatencyScore > models[j].LatencyScore
	})

	r := Report{Models: models, TotalTimeout: totalTimeouts}
	if len(models) > 0 {
		r.Fastest = models[0].Model
		r.Slowest = models[len(models)-1].Model
		var sumScore float64
		for _, m := range models {
			sumScore += m.LatencyScore
		}
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Latency Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No latency data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:     %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:           %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Fastest:              %s\n", r.Fastest)
	fmt.Fprintf(&b, "  Slowest:              %s\n", r.Slowest)
	fmt.Fprintf(&b, "  Total timeouts:       %d\n\n", r.TotalTimeout)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100  (%d samples, %d labs)\n",
			m.Model, m.Grade, m.LatencyScore, m.SampleCount, m.LabCount)
		fmt.Fprintf(&b, "    %-20s %.1f ms\n", "mean", m.Detail.MeanMS)
		fmt.Fprintf(&b, "    %-20s %.1f ms\n", "median", m.Detail.MedianMS)
		fmt.Fprintf(&b, "    %-20s %.1f ms\n", "p90", m.Detail.P90MS)
		fmt.Fprintf(&b, "    %-20s %.1f ms\n", "p95", m.Detail.P95MS)
		fmt.Fprintf(&b, "    %-20s %.1f ms\n", "p99", m.Detail.P99MS)
		fmt.Fprintf(&b, "    %-20s %.1f ms\n", "min", m.Detail.MinMS)
		fmt.Fprintf(&b, "    %-20s %.1f ms\n", "max", m.Detail.MaxMS)
		fmt.Fprintf(&b, "    %-20s %.1f ms\n", "stddev", m.Detail.StdDevMS)
		fmt.Fprintf(&b, "    Timeouts: %d  Slow (>%dms): %d\n",
			m.TimeoutCount, int(slowThresholdMS), m.SlowCount)
		if len(m.SlowPatterns) > 0 {
			b.WriteString("    Slowest patterns:\n")
			for _, sp := range m.SlowPatterns {
				fmt.Fprintf(&b, "      %.1fms  %s  %s\n", sp.LatencyMS, sp.Lab, sp.Summary)
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
