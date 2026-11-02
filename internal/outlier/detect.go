package outlier

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Outlier struct {
	Model   string  `json:"model"`
	Checker string  `json:"checker"`
	Score   float64 `json:"score"`
	Mean    float64 `json:"mean"`
	StdDev  float64 `json:"std_dev"`
	ZScore  float64 `json:"z_score"`
	Type    string  `json:"type"`
}

type OutlierReport struct {
	Outliers   []Outlier `json:"outliers"`
	Threshold  float64   `json:"threshold"`
	TotalPairs int       `json:"total_pairs"`
	OutlierPct float64   `json:"outlier_pct"`
}

type scorePair struct {
	model string
	score float64
}

func DetectOutliers(rr model.RunReport, findings []model.Finding, threshold float64) OutlierReport {
	if threshold <= 0 {
		threshold = 2.0
	}

	type key struct{ checker, model string }
	scores := map[key]float64{}
	checkerScores := map[string][]scorePair{}

	for _, r := range rr.Reflection {
		checker := r.Reviewer
		if checker == "" {
			continue
		}
		m := r.Model
		k := key{checker, m}
		if _, exists := scores[k]; !exists {
			score := float64(r.Final) / 10.0
			scores[k] = score
			checkerScores[checker] = append(checkerScores[checker], scorePair{m, score})
		}
	}

	for _, f := range findings {
		checker := f.Model.Provider
		if checker == "" {
			checker = f.Model.Model
		}
		m := f.Model.Model
		k := key{checker, m}
		if _, exists := scores[k]; !exists {
			score := severityScore(f.Severity)
			scores[k] = score
			checkerScores[checker] = append(checkerScores[checker], scorePair{m, score})
		}
	}

	var outliers []Outlier
	totalPairs := len(scores)

	for checker, pairs := range checkerScores {
		if len(pairs) < 2 {
			continue
		}
		mean, stddev := meanStdDev(pairs)
		if stddev == 0 {
			continue
		}
		for _, p := range pairs {
			z := (p.score - mean) / stddev
			if math.Abs(z) >= threshold {
				t := "low"
				if z > 0 {
					t = "high"
				}
				outliers = append(outliers, Outlier{
					Model:   p.model,
					Checker: checker,
					Score:   p.score,
					Mean:    math.Round(mean*1000) / 1000,
					StdDev:  math.Round(stddev*1000) / 1000,
					ZScore:  math.Round(z*1000) / 1000,
					Type:    t,
				})
			}
		}
	}

	sort.Slice(outliers, func(i, j int) bool {
		return math.Abs(outliers[i].ZScore) > math.Abs(outliers[j].ZScore)
	})

	pct := 0.0
	if totalPairs > 0 {
		pct = float64(len(outliers)) / float64(totalPairs) * 100
	}

	return OutlierReport{
		Outliers:   outliers,
		Threshold:  threshold,
		TotalPairs: totalPairs,
		OutlierPct: math.Round(pct*100) / 100,
	}
}

func severityScore(s model.Severity) float64 {
	switch s {
	case model.SevCritical:
		return 1.0
	case model.SevHigh:
		return 0.75
	case model.SevMedium:
		return 0.5
	case model.SevLow:
		return 0.25
	default:
		return 0.5
	}
}

func meanStdDev(pairs []scorePair) (float64, float64) {
	n := float64(len(pairs))
	sum := 0.0
	for _, p := range pairs {
		sum += p.score
	}
	mean := sum / n

	variance := 0.0
	for _, p := range pairs {
		d := p.score - mean
		variance += d * d
	}
	variance /= n

	return mean, math.Sqrt(variance)
}

func FormatText(r OutlierReport) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Outlier Detection (threshold: %.1f sigma, %d pairs, %.1f%% outliers)\n\n",
		r.Threshold, r.TotalPairs, r.OutlierPct))

	if len(r.Outliers) == 0 {
		b.WriteString("No outliers detected.\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("%-20s %-20s %8s %8s %8s %8s %5s\n",
		"Model", "Checker", "Score", "Mean", "StdDev", "Z-Score", "Type"))
	b.WriteString(strings.Repeat("-", 93))
	b.WriteString("\n")

	for _, o := range r.Outliers {
		model := o.Model
		if len(model) > 20 {
			model = model[:17] + "..."
		}
		checker := o.Checker
		if len(checker) > 20 {
			checker = checker[:17] + "..."
		}
		b.WriteString(fmt.Sprintf("%-20s %-20s %8.3f %8.3f %8.3f %8.3f %5s\n",
			model, checker, o.Score, o.Mean, o.StdDev, o.ZScore, o.Type))
	}

	return b.String()
}
