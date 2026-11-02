package sevdist

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type SevCount struct {
	Severity string  `json:"severity"`
	Count    int     `json:"count"`
	Pct      float64 `json:"pct"`
}

type ModelDist struct {
	Model      string     `json:"model"`
	Total      int        `json:"total"`
	Counts     []SevCount `json:"counts"`
	MeanRank   float64    `json:"mean_rank"`
	MedianSev  string     `json:"median_severity"`
	P90Sev     string     `json:"p90_severity"`
}

type Report struct {
	TotalFindings int        `json:"total_findings"`
	Overall       []SevCount `json:"overall"`
	ByModel       []ModelDist `json:"by_model"`
	MeanRank      float64    `json:"mean_rank"`
	MedianSev     string     `json:"median_severity"`
	P90Sev        string     `json:"p90_severity"`
	Recommendation string   `json:"recommendation"`
}

var sevOrder = []model.Severity{
	model.SevInfo, model.SevLow, model.SevMedium, model.SevHigh, model.SevCritical,
}

func sevLabel(s model.Severity) string {
	return string(s)
}

func rankFromSev(s model.Severity) float64 {
	return float64(s.Rank())
}

func sevFromRank(r int) string {
	for _, s := range sevOrder {
		if s.Rank() == r {
			return string(s)
		}
	}
	return "unknown"
}

func percentile(ranks []float64, p float64) float64 {
	if len(ranks) == 0 {
		return 0
	}
	sort.Float64s(ranks)
	idx := p / 100.0 * float64(len(ranks)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi || hi >= len(ranks) {
		return ranks[lo]
	}
	frac := idx - float64(lo)
	return ranks[lo]*(1-frac) + ranks[hi]*frac
}

func computeDist(findings []model.Finding) ([]SevCount, float64, string, string) {
	counts := make(map[string]int)
	var ranks []float64
	for _, f := range findings {
		counts[string(f.Severity)]++
		ranks = append(ranks, rankFromSev(f.Severity))
	}

	var dist []SevCount
	total := len(findings)
	for _, s := range sevOrder {
		c := counts[string(s)]
		if c == 0 {
			continue
		}
		pct := math.Round(float64(c)/float64(total)*1000) / 10
		dist = append(dist, SevCount{
			Severity: string(s),
			Count:    c,
			Pct:      pct,
		})
	}

	meanRank := 0.0
	if len(ranks) > 0 {
		sum := 0.0
		for _, r := range ranks {
			sum += r
		}
		meanRank = math.Round(sum/float64(len(ranks))*100) / 100
	}

	medianRank := int(math.Round(percentile(ranks, 50)))
	p90Rank := int(math.Round(percentile(ranks, 90)))

	return dist, meanRank, sevFromRank(medianRank), sevFromRank(p90Rank)
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{Recommendation: "No findings to analyze"}
	}

	overall, meanRank, medianSev, p90Sev := computeDist(findings)

	modelFindings := make(map[string][]model.Finding)
	for _, f := range findings {
		mdl := f.Model.Model
		modelFindings[mdl] = append(modelFindings[mdl], f)
	}

	var byModel []ModelDist
	for mdl, ff := range modelFindings {
		dist, mr, med, p90 := computeDist(ff)
		byModel = append(byModel, ModelDist{
			Model:     mdl,
			Total:     len(ff),
			Counts:    dist,
			MeanRank:  mr,
			MedianSev: med,
			P90Sev:    p90,
		})
	}
	sort.Slice(byModel, func(i, j int) bool {
		return byModel[i].Model < byModel[j].Model
	})

	rec := fmt.Sprintf("Median severity: %s, P90: %s, mean rank: %.2f", medianSev, p90Sev, meanRank)

	return Report{
		TotalFindings:  len(findings),
		Overall:        overall,
		ByModel:        byModel,
		MeanRank:       meanRank,
		MedianSev:      medianSev,
		P90Sev:         p90Sev,
		Recommendation: rec,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Severity Distribution Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	fmt.Fprintf(&b, "  Total findings:    %d\n", r.TotalFindings)
	fmt.Fprintf(&b, "  Mean severity:     %.2f\n", r.MeanRank)
	fmt.Fprintf(&b, "  Median severity:   %s\n", r.MedianSev)
	fmt.Fprintf(&b, "  P90 severity:      %s\n", r.P90Sev)
	fmt.Fprintf(&b, "  %s\n\n", r.Recommendation)

	if len(r.Overall) > 0 {
		b.WriteString("  Overall Distribution:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, sc := range r.Overall {
			bar := strings.Repeat("#", int(sc.Pct/2))
			fmt.Fprintf(&b, "    %-10s %4d (%5.1f%%) %s\n", sc.Severity, sc.Count, sc.Pct, bar)
		}
		b.WriteString("\n")
	}

	if len(r.ByModel) > 0 {
		b.WriteString("  By Model:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, md := range r.ByModel {
			fmt.Fprintf(&b, "    %s (%d findings, median=%s, P90=%s)\n",
				md.Model, md.Total, md.MedianSev, md.P90Sev)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
