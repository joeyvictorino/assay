package modeldrift

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type RunSnapshot struct {
	RunID    string          `json:"run_id"`
	Model    string          `json:"model"`
	Findings []model.Finding `json:"findings"`
}

type BehaviorMetrics struct {
	SeverityDistribution map[string]int `json:"severity_distribution"`
	ClassDistribution    map[string]int `json:"class_distribution"`
	MeanEvidenceCount    float64        `json:"mean_evidence_count"`
	TotalFindings        int            `json:"total_findings"`
	ValidationRate       float64        `json:"validation_rate"`
	RefusalRate          float64        `json:"refusal_rate"`
	UniqueCWEs           int            `json:"unique_cwes"`
}

type DriftDimension struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
	Delta string  `json:"delta"`
}

type RunComparison struct {
	RunA       string            `json:"run_a"`
	RunB       string            `json:"run_b"`
	ModelA     string            `json:"model_a"`
	ModelB     string            `json:"model_b"`
	Dimensions []DriftDimension  `json:"dimensions"`
	OverallDrift float64         `json:"overall_drift"`
	Flagged    bool              `json:"flagged"`
	MetricsA   BehaviorMetrics   `json:"metrics_a"`
	MetricsB   BehaviorMetrics   `json:"metrics_b"`
}

type Report struct {
	Comparisons []RunComparison `json:"comparisons"`
	Summary     Summary         `json:"summary"`
}

type Summary struct {
	TotalComparisons int     `json:"total_comparisons"`
	FlaggedCount     int     `json:"flagged_count"`
	MeanDrift        float64 `json:"mean_drift"`
	MaxDrift         float64 `json:"max_drift"`
	MostDrifted      string  `json:"most_drifted"`
}

func computeMetrics(findings []model.Finding) BehaviorMetrics {
	m := BehaviorMetrics{
		SeverityDistribution: make(map[string]int),
		ClassDistribution:    make(map[string]int),
	}
	if len(findings) == 0 {
		return m
	}

	m.TotalFindings = len(findings)
	cwes := make(map[string]bool)
	var totalEvidence int
	var validated, refused int

	for _, f := range findings {
		m.SeverityDistribution[string(f.Severity)]++
		m.ClassDistribution[string(f.Class)]++
		totalEvidence += len(f.Evidence)
		if f.CWE != "" {
			cwes[f.CWE] = true
		}
		if f.State == model.StateValidated {
			validated++
		}
		if f.State == model.StateDeclined {
			refused++
		}
	}

	m.MeanEvidenceCount = round2(float64(totalEvidence) / float64(len(findings)))
	m.ValidationRate = round2(float64(validated) / float64(len(findings)))
	m.RefusalRate = round2(float64(refused) / float64(len(findings)))
	m.UniqueCWEs = len(cwes)
	return m
}

func distributionDrift(a, b map[string]int, totalA, totalB int) float64 {
	if totalA == 0 && totalB == 0 {
		return 0
	}
	keys := make(map[string]bool)
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}

	var sum float64
	for k := range keys {
		pA := float64(a[k]) / math.Max(float64(totalA), 1)
		pB := float64(b[k]) / math.Max(float64(totalB), 1)
		sum += math.Abs(pA - pB)
	}
	return round2(math.Min(sum/2, 1.0))
}

func rateDrift(a, b float64) float64 {
	return round2(math.Abs(a - b))
}

func countDrift(a, b int) (float64, string) {
	if a == 0 && b == 0 {
		return 0, "0 -> 0"
	}
	maxVal := math.Max(float64(a), float64(b))
	drift := math.Abs(float64(a)-float64(b)) / maxVal
	return round2(drift), fmt.Sprintf("%d -> %d", a, b)
}

func compareRuns(a, b RunSnapshot, threshold float64) RunComparison {
	ma := computeMetrics(a.Findings)
	mb := computeMetrics(b.Findings)

	sevDrift := distributionDrift(ma.SeverityDistribution, mb.SeverityDistribution, ma.TotalFindings, mb.TotalFindings)
	classDrift := distributionDrift(ma.ClassDistribution, mb.ClassDistribution, ma.TotalFindings, mb.TotalFindings)
	countDriftVal, countDelta := countDrift(ma.TotalFindings, mb.TotalFindings)
	valDrift := rateDrift(ma.ValidationRate, mb.ValidationRate)
	refDrift := rateDrift(ma.RefusalRate, mb.RefusalRate)
	evDrift := rateDrift(ma.MeanEvidenceCount, mb.MeanEvidenceCount)
	cweDriftVal, cweDelta := countDrift(ma.UniqueCWEs, mb.UniqueCWEs)

	dims := []DriftDimension{
		{Name: "severity_distribution", Score: sevDrift, Delta: formatDist(ma.SeverityDistribution, mb.SeverityDistribution)},
		{Name: "class_distribution", Score: classDrift, Delta: formatDist(ma.ClassDistribution, mb.ClassDistribution)},
		{Name: "finding_count", Score: countDriftVal, Delta: countDelta},
		{Name: "validation_rate", Score: valDrift, Delta: fmt.Sprintf("%.2f -> %.2f", ma.ValidationRate, mb.ValidationRate)},
		{Name: "refusal_rate", Score: refDrift, Delta: fmt.Sprintf("%.2f -> %.2f", ma.RefusalRate, mb.RefusalRate)},
		{Name: "evidence_depth", Score: evDrift, Delta: fmt.Sprintf("%.1f -> %.1f", ma.MeanEvidenceCount, mb.MeanEvidenceCount)},
		{Name: "cwe_coverage", Score: cweDriftVal, Delta: cweDelta},
	}

	var total float64
	for _, d := range dims {
		total += d.Score
	}
	overall := round2(total / float64(len(dims)))

	return RunComparison{
		RunA:         a.RunID,
		RunB:         b.RunID,
		ModelA:       a.Model,
		ModelB:       b.Model,
		Dimensions:   dims,
		OverallDrift: overall,
		Flagged:      overall >= threshold,
		MetricsA:     ma,
		MetricsB:     mb,
	}
}

func formatDist(a, b map[string]int) string {
	keys := make(map[string]bool)
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	parts := make([]string, 0, len(sorted))
	for _, k := range sorted {
		parts = append(parts, fmt.Sprintf("%s:%d->%d", k, a[k], b[k]))
	}
	return strings.Join(parts, " ")
}

func Analyze(snapshots []RunSnapshot, threshold float64) Report {
	if threshold <= 0 {
		threshold = 0.3
	}

	var comparisons []RunComparison
	for i := 0; i < len(snapshots)-1; i++ {
		c := compareRuns(snapshots[i], snapshots[i+1], threshold)
		comparisons = append(comparisons, c)
	}

	var flagged int
	var totalDrift, maxDrift float64
	var mostDrifted string
	for _, c := range comparisons {
		if c.Flagged {
			flagged++
		}
		totalDrift += c.OverallDrift
		if c.OverallDrift > maxDrift {
			maxDrift = c.OverallDrift
			mostDrifted = fmt.Sprintf("%s vs %s", c.RunA, c.RunB)
		}
	}

	var meanDrift float64
	if len(comparisons) > 0 {
		meanDrift = round2(totalDrift / float64(len(comparisons)))
	}

	return Report{
		Comparisons: comparisons,
		Summary: Summary{
			TotalComparisons: len(comparisons),
			FlaggedCount:     flagged,
			MeanDrift:        meanDrift,
			MaxDrift:         round2(maxDrift),
			MostDrifted:      mostDrifted,
		},
	}
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Cross-Run Model Behavior Drift\n")
	b.WriteString(strings.Repeat("=", 40) + "\n")
	fmt.Fprintf(&b, "Comparisons:   %d\n", r.Summary.TotalComparisons)
	fmt.Fprintf(&b, "Flagged:       %d\n", r.Summary.FlaggedCount)
	fmt.Fprintf(&b, "Mean drift:    %.2f\n", r.Summary.MeanDrift)
	fmt.Fprintf(&b, "Max drift:     %.2f\n", r.Summary.MaxDrift)
	if r.Summary.MostDrifted != "" {
		fmt.Fprintf(&b, "Most drifted:  %s\n", r.Summary.MostDrifted)
	}
	b.WriteString("\n")

	for _, c := range r.Comparisons {
		flag := ""
		if c.Flagged {
			flag = " [FLAGGED]"
		}
		fmt.Fprintf(&b, "  %s (%s) vs %s (%s)  drift=%.2f%s\n",
			c.RunA, c.ModelA, c.RunB, c.ModelB, c.OverallDrift, flag)
		for _, d := range c.Dimensions {
			if d.Score > 0 {
				fmt.Fprintf(&b, "    %-24s %.2f  %s\n", d.Name, d.Score, d.Delta)
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
