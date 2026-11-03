package promptlength

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type LengthBucket struct {
	Label        string  `json:"label"`
	MinChars     int     `json:"min_chars"`
	MaxChars     int     `json:"max_chars"`
	Count        int     `json:"count"`
	AvgFindings  float64 `json:"avg_findings"`
	AvgSeverity  float64 `json:"avg_severity"`
	ValidatedPct float64 `json:"validated_pct"`
}

type ModelImpact struct {
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	TotalFindings    int     `json:"total_findings"`
	AvgPromptLen     float64 `json:"avg_prompt_len"`
	ShortPromptPct   float64 `json:"short_prompt_pct"`
	LongPromptPct    float64 `json:"long_prompt_pct"`
	Correlation      float64 `json:"length_severity_correlation"`
	OptimalRange     string  `json:"optimal_range"`
}

type Summary struct {
	TotalFindings    int     `json:"total_findings"`
	WithPromptHash   int     `json:"with_prompt_hash"`
	AvgSummaryLen    int     `json:"avg_summary_len"`
	MedianSummaryLen int     `json:"median_summary_len"`
	ShortCount       int     `json:"short_count"`
	MediumCount      int     `json:"medium_count"`
	LongCount        int     `json:"long_count"`
	VeryLongCount    int     `json:"very_long_count"`
}

type Report struct {
	Buckets []LengthBucket `json:"buckets"`
	Models  []ModelImpact  `json:"models"`
	Summary Summary        `json:"summary"`
}

const (
	shortThreshold    = 100
	mediumThreshold   = 280
	longThreshold     = 500
	veryLongThreshold = 1000
)

type entry struct {
	length   int
	severity int
	model    string
	provider string
	state    model.FindingState
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	var entries []entry
	withHash := 0

	for _, f := range findings {
		length := len(f.Summary)
		sev := severityToInt(f.Severity)

		if f.PromptHash != "" {
			withHash++
		}

		entries = append(entries, entry{
			length:   length,
			severity: sev,
			model:    f.Model.Model,
			provider: f.Model.Provider,
			state:    f.State,
		})
	}

	bucketDefs := []struct {
		label    string
		minChars int
		maxChars int
	}{
		{"short (<100)", 0, shortThreshold},
		{"medium (100-280)", shortThreshold, mediumThreshold},
		{"long (280-500)", mediumThreshold, longThreshold},
		{"very-long (>500)", longThreshold, math.MaxInt32},
	}

	buckets := make([]LengthBucket, len(bucketDefs))
	for i, bd := range bucketDefs {
		var sevSum, valCount, totalCount int
		for _, e := range entries {
			if e.length >= bd.minChars && e.length < bd.maxChars {
				totalCount++
				sevSum += e.severity
				if e.state == model.StateValidated {
					valCount++
				}
			}
		}
		avgSev := 0.0
		valPct := 0.0
		if totalCount > 0 {
			avgSev = round2(float64(sevSum) / float64(totalCount))
			valPct = round2(float64(valCount) / float64(totalCount) * 100)
		}
		buckets[i] = LengthBucket{
			Label:        bd.label,
			MinChars:     bd.minChars,
			MaxChars:     bd.maxChars,
			Count:        totalCount,
			AvgFindings:  round2(float64(totalCount)),
			AvgSeverity:  avgSev,
			ValidatedPct: valPct,
		}
	}

	byModel := map[string][]entry{}
	for _, e := range entries {
		if e.model != "" {
			byModel[e.model] = append(byModel[e.model], e)
		}
	}

	var models []ModelImpact
	for name, ee := range byModel {
		var lengths []float64
		var severities []float64
		shortCount, longCount := 0, 0
		provider := ""

		for _, e := range ee {
			lengths = append(lengths, float64(e.length))
			severities = append(severities, float64(e.severity))
			if e.length < shortThreshold {
				shortCount++
			}
			if e.length >= longThreshold {
				longCount++
			}
			if e.provider != "" {
				provider = e.provider
			}
		}

		avgLen := mean(lengths)
		corr := correlation(lengths, severities)

		optimal := classifyOptimalRange(ee)

		models = append(models, ModelImpact{
			Model:          name,
			Provider:       provider,
			TotalFindings:  len(ee),
			AvgPromptLen:   round2(avgLen),
			ShortPromptPct: round2(float64(shortCount) / float64(len(ee)) * 100),
			LongPromptPct:  round2(float64(longCount) / float64(len(ee)) * 100),
			Correlation:    round2(corr),
			OptimalRange:   optimal,
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].TotalFindings > models[j].TotalFindings
	})

	var allLengths []int
	for _, e := range entries {
		allLengths = append(allLengths, e.length)
	}
	sort.Ints(allLengths)

	shortC, medC, longC, vlC := 0, 0, 0, 0
	sumLen := 0
	for _, l := range allLengths {
		sumLen += l
		switch {
		case l < shortThreshold:
			shortC++
		case l < mediumThreshold:
			medC++
		case l < longThreshold:
			longC++
		default:
			vlC++
		}
	}

	avgL := 0
	medL := 0
	if len(allLengths) > 0 {
		avgL = sumLen / len(allLengths)
		medL = allLengths[len(allLengths)/2]
	}

	return Report{
		Buckets: buckets,
		Models:  models,
		Summary: Summary{
			TotalFindings:    len(entries),
			WithPromptHash:   withHash,
			AvgSummaryLen:    avgL,
			MedianSummaryLen: medL,
			ShortCount:       shortC,
			MediumCount:      medC,
			LongCount:        longC,
			VeryLongCount:    vlC,
		},
	}
}

func severityToInt(s model.Severity) int {
	switch s {
	case model.SevCritical:
		return 4
	case model.SevHigh:
		return 3
	case model.SevMedium:
		return 2
	case model.SevLow:
		return 1
	default:
		return 0
	}
}

func classifyOptimalRange(entries []entry) string {
	type bucket struct {
		label string
		min   int
		max   int
		val   int
		total int
	}
	bb := []bucket{
		{"short", 0, shortThreshold, 0, 0},
		{"medium", shortThreshold, mediumThreshold, 0, 0},
		{"long", mediumThreshold, longThreshold, 0, 0},
		{"very-long", longThreshold, math.MaxInt32, 0, 0},
	}
	for _, e := range entries {
		for i := range bb {
			if e.length >= bb[i].min && e.length < bb[i].max {
				bb[i].total++
				if e.state == model.StateValidated {
					bb[i].val++
				}
				break
			}
		}
	}
	bestPct := 0.0
	bestLabel := "medium"
	for _, b := range bb {
		if b.total >= 2 {
			pct := float64(b.val) / float64(b.total)
			if pct > bestPct {
				bestPct = pct
				bestLabel = b.label
			}
		}
	}
	return bestLabel
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

func correlation(xs, ys []float64) float64 {
	if len(xs) != len(ys) || len(xs) < 3 {
		return 0
	}
	n := float64(len(xs))
	mx, my := mean(xs), mean(ys)

	var num, dx2, dy2 float64
	for i := range xs {
		dx := xs[i] - mx
		dy := ys[i] - my
		num += dx * dy
		dx2 += dx * dx
		dy2 += dy * dy
	}
	denom := math.Sqrt(dx2 * dy2)
	if denom == 0 {
		return 0
	}
	_ = n
	return num / denom
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Prompt Length Impact Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")
	b.WriteString(fmt.Sprintf("  Total findings:     %d\n", r.Summary.TotalFindings))
	b.WriteString(fmt.Sprintf("  With prompt hash:   %d\n", r.Summary.WithPromptHash))
	b.WriteString(fmt.Sprintf("  Avg summary length: %d chars\n", r.Summary.AvgSummaryLen))
	b.WriteString(fmt.Sprintf("  Median length:      %d chars\n", r.Summary.MedianSummaryLen))
	b.WriteByte('\n')

	b.WriteString("  Length Distribution:\n")
	b.WriteString(fmt.Sprintf("    Short (<100):     %d\n", r.Summary.ShortCount))
	b.WriteString(fmt.Sprintf("    Medium (100-280): %d\n", r.Summary.MediumCount))
	b.WriteString(fmt.Sprintf("    Long (280-500):   %d\n", r.Summary.LongCount))
	b.WriteString(fmt.Sprintf("    Very long (>500): %d\n\n", r.Summary.VeryLongCount))

	if len(r.Buckets) > 0 {
		b.WriteString(fmt.Sprintf("  %-22s %6s %10s %10s\n", "Bucket", "Count", "AvgSev", "Validated%"))
		b.WriteString("  " + strings.Repeat("-", 52) + "\n")
		for _, bk := range r.Buckets {
			b.WriteString(fmt.Sprintf("  %-22s %6d %10.1f %9.1f%%\n",
				bk.Label, bk.Count, bk.AvgSeverity, bk.ValidatedPct))
		}
		b.WriteByte('\n')
	}

	if len(r.Models) > 0 {
		b.WriteString("  Per-Model Impact:\n")
		for _, m := range r.Models {
			b.WriteString(fmt.Sprintf("    %s (%s): %d findings, avg len %.0f, corr %.2f, optimal: %s\n",
				m.Model, m.Provider, m.TotalFindings, m.AvgPromptLen, m.Correlation, m.OptimalRange))
		}
		b.WriteByte('\n')
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
