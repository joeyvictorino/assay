package bias

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type BiasScore struct {
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
	Detected    bool    `json:"detected"`
}

type BiasReport struct {
	TotalFindings int         `json:"total_findings"`
	Biases        []BiasScore `json:"biases"`
	OverallRisk   string      `json:"overall_risk"`
}

func AnalyzeBias(findingsPath string) (BiasReport, error) {
	data, err := os.ReadFile(findingsPath)
	if err != nil {
		return BiasReport{}, fmt.Errorf("reading %s: %w", findingsPath, err)
	}
	var findings []model.Finding
	if err := json.Unmarshal(data, &findings); err != nil {
		return BiasReport{}, fmt.Errorf("parsing %s: %w", findingsPath, err)
	}
	return AnalyzeBiasFromFindings(findings), nil
}

func AnalyzeBiasFromFindings(findings []model.Finding) BiasReport {
	if len(findings) == 0 {
		return BiasReport{
			OverallRisk: "none",
			Biases: []BiasScore{
				{Name: "severity-clustering", Score: 0, Description: "No findings to analyze"},
				{Name: "positional-bias", Score: 0, Description: "No findings to analyze"},
				{Name: "anchoring-bias", Score: 0, Description: "No findings to analyze"},
				{Name: "category-bias", Score: 0, Description: "No findings to analyze"},
			},
		}
	}

	biases := []BiasScore{
		severityClustering(findings),
		positionalBias(findings),
		anchoringBias(findings),
		categoryBias(findings),
	}

	maxScore := 0.0
	for _, b := range biases {
		if b.Score > maxScore {
			maxScore = b.Score
		}
	}

	risk := "low"
	if maxScore >= 0.7 {
		risk = "high"
	} else if maxScore >= 0.4 {
		risk = "medium"
	}

	return BiasReport{
		TotalFindings: len(findings),
		Biases:        biases,
		OverallRisk:   risk,
	}
}

func severityClustering(findings []model.Finding) BiasScore {
	counts := map[model.Severity]int{}
	for _, f := range findings {
		counts[f.Severity]++
	}
	maxCount := 0
	for _, c := range counts {
		if c > maxCount {
			maxCount = c
		}
	}
	ratio := float64(maxCount) / float64(len(findings))
	score := 0.0
	if len(counts) == 1 {
		score = 1.0
	} else if ratio > 0.8 {
		score = 0.8
	} else if ratio > 0.6 {
		score = 0.4
	}
	score = math.Round(score*1000) / 1000
	desc := fmt.Sprintf("%d severity levels used, max concentration %.0f%%", len(counts), ratio*100)
	return BiasScore{
		Name:        "severity-clustering",
		Score:       score,
		Description: desc,
		Detected:    score >= 0.4,
	}
}

func positionalBias(findings []model.Finding) BiasScore {
	if len(findings) < 4 {
		return BiasScore{
			Name:        "positional-bias",
			Score:       0,
			Description: "Too few findings to detect positional bias",
		}
	}

	n := len(findings)
	quarter := n / 4
	if quarter == 0 {
		quarter = 1
	}

	sevRank := func(s model.Severity) int {
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

	firstAvg := 0.0
	for i := 0; i < quarter; i++ {
		firstAvg += float64(sevRank(findings[i].Severity))
	}
	firstAvg /= float64(quarter)

	lastAvg := 0.0
	for i := n - quarter; i < n; i++ {
		lastAvg += float64(sevRank(findings[i].Severity))
	}
	lastAvg /= float64(quarter)

	midAvg := 0.0
	midStart := quarter
	midEnd := n - quarter
	if midEnd <= midStart {
		midEnd = midStart + 1
	}
	midCount := 0
	for i := midStart; i < midEnd && i < n; i++ {
		midAvg += float64(sevRank(findings[i].Severity))
		midCount++
	}
	if midCount > 0 {
		midAvg /= float64(midCount)
	}

	diff := math.Abs(firstAvg-midAvg) + math.Abs(lastAvg-midAvg)
	score := math.Min(diff/4.0, 1.0)
	score = math.Round(score*1000) / 1000

	desc := fmt.Sprintf("First-quarter avg severity %.1f, mid %.1f, last-quarter %.1f", firstAvg, midAvg, lastAvg)
	return BiasScore{
		Name:        "positional-bias",
		Score:       score,
		Description: desc,
		Detected:    score >= 0.4,
	}
}

func anchoringBias(findings []model.Finding) BiasScore {
	if len(findings) < 3 {
		return BiasScore{
			Name:        "anchoring-bias",
			Score:       0,
			Description: "Too few findings to detect anchoring",
		}
	}

	firstSev := findings[0].Severity
	matchCount := 0
	for i := 1; i < len(findings); i++ {
		if findings[i].Severity == firstSev {
			matchCount++
		}
	}
	ratio := float64(matchCount) / float64(len(findings)-1)
	score := 0.0
	if ratio > 0.8 {
		score = 0.8
	} else if ratio > 0.6 {
		score = 0.4
	}
	score = math.Round(score*1000) / 1000

	desc := fmt.Sprintf("%.0f%% of subsequent findings match first finding severity (%s)", ratio*100, firstSev)
	return BiasScore{
		Name:        "anchoring-bias",
		Score:       score,
		Description: desc,
		Detected:    score >= 0.4,
	}
}

func categoryBias(findings []model.Finding) BiasScore {
	counts := map[model.Class]int{}
	for _, f := range findings {
		counts[f.Class]++
	}

	if len(counts) <= 1 {
		return BiasScore{
			Name:        "category-bias",
			Score:       1.0,
			Description: "All findings in a single category",
			Detected:    true,
		}
	}

	vals := make([]int, 0, len(counts))
	for _, c := range counts {
		vals = append(vals, c)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(vals)))

	topRatio := float64(vals[0]) / float64(len(findings))
	score := 0.0
	if topRatio > 0.8 {
		score = 0.8
	} else if topRatio > 0.6 {
		score = 0.4
	}
	score = math.Round(score*1000) / 1000

	desc := fmt.Sprintf("%d categories, top category %.0f%% of findings", len(counts), topRatio*100)
	return BiasScore{
		Name:        "category-bias",
		Score:       score,
		Description: desc,
		Detected:    score >= 0.4,
	}
}

func FormatText(r BiasReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Bias Analysis (%d findings, risk: %s)\n\n", r.TotalFindings, r.OverallRisk)
	for _, bs := range r.Biases {
		marker := " "
		if bs.Detected {
			marker = "!"
		}
		fmt.Fprintf(&b, "  [%s] %-22s score=%.2f  %s\n", marker, bs.Name, bs.Score, bs.Description)
	}
	return b.String()
}
