package modellatency

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Bucket struct {
	Label   string  `json:"label"`
	MinMS   int64   `json:"min_ms"`
	MaxMS   int64   `json:"max_ms"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

type ModelDistribution struct {
	Model    string   `json:"model"`
	Provider string   `json:"provider"`
	Count    int      `json:"count"`
	MeanMS   float64  `json:"mean_ms"`
	MedianMS float64  `json:"median_ms"`
	P90MS    float64  `json:"p90_ms"`
	P95MS    float64  `json:"p95_ms"`
	P99MS    float64  `json:"p99_ms"`
	StdDev   float64  `json:"std_dev_ms"`
	Skewness float64  `json:"skewness"`
	Shape    string   `json:"shape"`
	Buckets  []Bucket `json:"buckets"`
}

type Summary struct {
	TotalModels  int     `json:"total_models"`
	TotalSamples int     `json:"total_samples"`
	GlobalMeanMS float64 `json:"global_mean_ms"`
	GlobalP95MS  float64 `json:"global_p95_ms"`
	FastestModel string  `json:"fastest_model"`
	SlowestModel string  `json:"slowest_model"`
	SpreadRatio  float64 `json:"spread_ratio"`
}

type Report struct {
	Models  []ModelDistribution `json:"models"`
	Summary Summary             `json:"summary"`
}

var defaultBuckets = []struct {
	label string
	minMS int64
	maxMS int64
}{
	{"<100ms", 0, 100},
	{"100-500ms", 100, 500},
	{"500ms-1s", 500, 1000},
	{"1-2s", 1000, 2000},
	{"2-5s", 2000, 5000},
	{"5-10s", 5000, 10000},
	{">10s", 10000, math.MaxInt64},
}

func Analyze(findings []model.Finding) Report {
	byModel := map[string][]int64{}
	providerOf := map[string]string{}

	for _, f := range findings {
		for _, ev := range f.Evidence {
			if ev.StatusCode <= 0 {
				continue
			}
			key := f.Model.Model
			if key == "" {
				continue
			}
			lat := int64(ev.StatusCode)
			byModel[key] = append(byModel[key], lat)
			if f.Model.Provider != "" {
				providerOf[key] = f.Model.Provider
			}
		}
	}

	if len(byModel) == 0 {
		return Report{}
	}

	var allLatencies []float64
	var models []ModelDistribution
	for name, lats := range byModel {
		sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
		floats := toFloat64(lats)
		allLatencies = append(allLatencies, floats...)

		avg := mean(floats)
		sd := stddev(floats, avg)
		sk := skewness(floats, avg, sd)

		buckets := makeBuckets(lats)

		models = append(models, ModelDistribution{
			Model:    name,
			Provider: providerOf[name],
			Count:    len(lats),
			MeanMS:   round2(avg),
			MedianMS: round2(percentile(floats, 50)),
			P90MS:    round2(percentile(floats, 90)),
			P95MS:    round2(percentile(floats, 95)),
			P99MS:    round2(percentile(floats, 99)),
			StdDev:   round2(sd),
			Skewness: round2(sk),
			Shape:    classifyShape(sk),
			Buckets:  buckets,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].MeanMS < models[j].MeanMS
	})

	sort.Float64s(allLatencies)
	globalMean := mean(allLatencies)

	fastest, slowest := "", ""
	if len(models) > 0 {
		fastest = models[0].Model
		slowest = models[len(models)-1].Model
	}

	spread := 0.0
	if len(models) >= 2 && models[0].MeanMS > 0 {
		spread = round2(models[len(models)-1].MeanMS / models[0].MeanMS)
	}

	return Report{
		Models: models,
		Summary: Summary{
			TotalModels:  len(models),
			TotalSamples: len(allLatencies),
			GlobalMeanMS: round2(globalMean),
			GlobalP95MS:  round2(percentile(allLatencies, 95)),
			FastestModel: fastest,
			SlowestModel: slowest,
			SpreadRatio:  spread,
		},
	}
}

func makeBuckets(sorted []int64) []Bucket {
	total := len(sorted)
	if total == 0 {
		return nil
	}
	buckets := make([]Bucket, len(defaultBuckets))
	for i, b := range defaultBuckets {
		buckets[i] = Bucket{Label: b.label, MinMS: b.minMS, MaxMS: b.maxMS}
	}
	for _, v := range sorted {
		for i := range buckets {
			if v >= buckets[i].MinMS && v < buckets[i].MaxMS {
				buckets[i].Count++
				break
			}
		}
	}
	for i := range buckets {
		buckets[i].Percent = round2(float64(buckets[i].Count) / float64(total) * 100)
	}
	return buckets
}

func classifyShape(sk float64) string {
	switch {
	case sk > 1.0:
		return "right-skewed"
	case sk < -1.0:
		return "left-skewed"
	case sk > 0.5:
		return "moderately-right-skewed"
	case sk < -0.5:
		return "moderately-left-skewed"
	default:
		return "symmetric"
	}
}

func toFloat64(vals []int64) []float64 {
	out := make([]float64, len(vals))
	for i, v := range vals {
		out[i] = float64(v)
	}
	return out
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

func skewness(vals []float64, m, sd float64) float64 {
	if len(vals) < 3 || sd == 0 {
		return 0
	}
	n := float64(len(vals))
	sum := 0.0
	for _, v := range vals {
		d := (v - m) / sd
		sum += d * d * d
	}
	return sum / n
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

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Model Response Time Distribution\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")
	b.WriteString(fmt.Sprintf("  Models:       %d\n", r.Summary.TotalModels))
	b.WriteString(fmt.Sprintf("  Samples:      %d\n", r.Summary.TotalSamples))
	b.WriteString(fmt.Sprintf("  Global Mean:  %.0f ms\n", r.Summary.GlobalMeanMS))
	b.WriteString(fmt.Sprintf("  Global P95:   %.0f ms\n", r.Summary.GlobalP95MS))
	if r.Summary.FastestModel != "" {
		b.WriteString(fmt.Sprintf("  Fastest:      %s\n", r.Summary.FastestModel))
		b.WriteString(fmt.Sprintf("  Slowest:      %s\n", r.Summary.SlowestModel))
		b.WriteString(fmt.Sprintf("  Spread:       %.1fx\n", r.Summary.SpreadRatio))
	}
	b.WriteByte('\n')

	for _, m := range r.Models {
		b.WriteString(fmt.Sprintf("  %s (%s) — %d samples, shape: %s\n", m.Model, m.Provider, m.Count, m.Shape))
		b.WriteString(fmt.Sprintf("    Mean: %.0f ms  Median: %.0f ms  P95: %.0f ms  P99: %.0f ms  StdDev: %.0f ms\n",
			m.MeanMS, m.MedianMS, m.P95MS, m.P99MS, m.StdDev))
		b.WriteString("    Distribution:\n")
		for _, bkt := range m.Buckets {
			if bkt.Count == 0 {
				continue
			}
			bar := strings.Repeat("#", int(bkt.Percent/2))
			b.WriteString(fmt.Sprintf("      %-12s %4d (%5.1f%%) %s\n", bkt.Label, bkt.Count, bkt.Percent, bar))
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
