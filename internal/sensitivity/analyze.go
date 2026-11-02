package sensitivity

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PromptVariant struct {
	Hash    string  `json:"hash"`
	Preview string  `json:"preview"`
	Score   float64 `json:"score"`
}

type CheckerSensitivity struct {
	Checker   string          `json:"checker"`
	Variants  []PromptVariant `json:"variants"`
	MeanScore float64         `json:"mean_score"`
	StdDev    float64         `json:"std_dev"`
	Range     float64         `json:"range"`
	CV        float64         `json:"cv"`
	Rating    string          `json:"rating"`
}

type SensitivityReport struct {
	Checkers      []CheckerSensitivity `json:"checkers"`
	MostStable    string               `json:"most_stable"`
	MostSensitive string               `json:"most_sensitive"`
	OverallCV     float64              `json:"overall_cv"`
}

type variantKey struct {
	checker string
	model   string
}

func Analyze(rr model.RunReport) SensitivityReport {
	groups := map[string][]PromptVariant{}

	for _, ref := range rr.Reflection {
		if !ref.Scored || ref.Final == 0 {
			continue
		}
		key := ref.Reviewer
		if key == "" {
			key = ref.Model
		}
		hash := ref.PromptVariant
		if hash == "" {
			hash = fmt.Sprintf("default-%s", ref.Lab)
		}
		preview := ref.Lab
		if ref.PromptVariant != "" {
			preview = ref.PromptVariant
		}
		if len(preview) > 80 {
			preview = preview[:80]
		}
		groups[key] = append(groups[key], PromptVariant{
			Hash:    hash,
			Preview: preview,
			Score:   float64(ref.Final),
		})
	}

	var checkers []CheckerSensitivity
	for name, variants := range groups {
		if len(variants) < 2 {
			checkers = append(checkers, CheckerSensitivity{
				Checker:   name,
				Variants:  variants,
				MeanScore: variants[0].Score,
				Rating:    "stable",
			})
			continue
		}
		cs := computeSensitivity(name, variants)
		checkers = append(checkers, cs)
	}

	sort.Slice(checkers, func(i, j int) bool {
		return checkers[i].CV < checkers[j].CV
	})

	report := SensitivityReport{Checkers: checkers}
	if len(checkers) > 0 {
		report.MostStable = checkers[0].Checker
		report.MostSensitive = checkers[len(checkers)-1].Checker
	}

	var totalCV float64
	for _, c := range checkers {
		totalCV += c.CV
	}
	if len(checkers) > 0 {
		report.OverallCV = totalCV / float64(len(checkers))
	}

	return report
}

func computeSensitivity(name string, variants []PromptVariant) CheckerSensitivity {
	n := float64(len(variants))
	var sum float64
	minScore := math.Inf(1)
	maxScore := math.Inf(-1)
	for _, v := range variants {
		sum += v.Score
		if v.Score < minScore {
			minScore = v.Score
		}
		if v.Score > maxScore {
			maxScore = v.Score
		}
	}
	mean := sum / n

	var sqDiffSum float64
	for _, v := range variants {
		d := v.Score - mean
		sqDiffSum += d * d
	}
	stddev := math.Sqrt(sqDiffSum / n)

	var cv float64
	if mean != 0 {
		cv = stddev / math.Abs(mean)
	}

	return CheckerSensitivity{
		Checker:   name,
		Variants:  variants,
		MeanScore: mean,
		StdDev:    stddev,
		Range:     maxScore - minScore,
		CV:        cv,
		Rating:    rateCV(cv),
	}
}

func rateCV(cv float64) string {
	switch {
	case cv < 0.1:
		return "stable"
	case cv < 0.2:
		return "moderate"
	case cv < 0.3:
		return "sensitive"
	default:
		return "highly_sensitive"
	}
}

func FormatText(r SensitivityReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Prompt Sensitivity Analysis (%d checkers)\n\n", len(r.Checkers))
	fmt.Fprintf(&b, "%-25s %8s %8s %8s %8s  %s\n", "Checker", "Mean", "StdDev", "Range", "CV", "Rating")
	fmt.Fprintln(&b, strings.Repeat("-", 80))
	for _, c := range r.Checkers {
		fmt.Fprintf(&b, "%-25s %8.2f %8.2f %8.2f %8.4f  %s\n",
			truncate(c.Checker, 25), c.MeanScore, c.StdDev, c.Range, c.CV, c.Rating)
	}
	fmt.Fprintln(&b)
	if r.MostStable != "" {
		fmt.Fprintf(&b, "Most stable:    %s\n", r.MostStable)
		fmt.Fprintf(&b, "Most sensitive: %s\n", r.MostSensitive)
	}
	fmt.Fprintf(&b, "Overall CV:     %.4f\n", r.OverallCV)
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
