package sevdist

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type SeverityBucket struct {
	Severity string  `json:"severity"`
	Count    int     `json:"count"`
	Pct      float64 `json:"pct"`
}

type ModelDist struct {
	Model   string           `json:"model"`
	Total   int              `json:"total"`
	Buckets []SeverityBucket `json:"buckets"`
	Median  string           `json:"median_severity"`
	Mean    float64          `json:"mean_rank"`
}

type Report struct {
	TotalFindings  int              `json:"total_findings"`
	Overall        []SeverityBucket `json:"overall"`
	ByModel        []ModelDist      `json:"by_model"`
	MedianSeverity string           `json:"median_severity"`
	MeanRank       float64          `json:"mean_rank"`
	Recommendation string           `json:"recommendation"`
}

var sevOrder = []model.Severity{
	model.SevInfo, model.SevLow, model.SevMedium, model.SevHigh, model.SevCritical,
}

func medianSeverity(ranks []int) string {
	if len(ranks) == 0 {
		return "none"
	}
	sort.Ints(ranks)
	mid := ranks[len(ranks)/2]
	for _, s := range sevOrder {
		if s.Rank() == mid {
			return string(s)
		}
	}
	return "unknown"
}

func buildBuckets(counts map[model.Severity]int, total int) []SeverityBucket {
	var buckets []SeverityBucket
	for _, s := range sevOrder {
		c := counts[s]
		if c == 0 {
			continue
		}
		pct := 0.0
		if total > 0 {
			pct = math.Round(float64(c)/float64(total)*1000) / 1000
		}
		buckets = append(buckets, SeverityBucket{
			Severity: string(s),
			Count:    c,
			Pct:      pct,
		})
	}
	return buckets
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{Recommendation: "No findings to analyze"}
	}

	overallCounts := make(map[model.Severity]int)
	modelCounts := make(map[string]map[model.Severity]int)
	var allRanks []int

	for _, f := range findings {
		overallCounts[f.Severity]++
		allRanks = append(allRanks, f.Severity.Rank())

		mdl := f.Model.Model
		if mdl == "" {
			mdl = "unknown"
		}
		if modelCounts[mdl] == nil {
			modelCounts[mdl] = make(map[model.Severity]int)
		}
		modelCounts[mdl][f.Severity]++
	}

	total := len(findings)
	overall := buildBuckets(overallCounts, total)

	meanRank := 0.0
	for _, r := range allRanks {
		meanRank += float64(r)
	}
	meanRank = math.Round(meanRank/float64(total)*100) / 100

	var byModel []ModelDist
	for mdl, counts := range modelCounts {
		mTotal := 0
		var mRanks []int
		for sev, c := range counts {
			mTotal += c
			for i := 0; i < c; i++ {
				mRanks = append(mRanks, sev.Rank())
			}
		}
		mMean := 0.0
		for _, r := range mRanks {
			mMean += float64(r)
		}
		mMean = math.Round(mMean/float64(mTotal)*100) / 100

		byModel = append(byModel, ModelDist{
			Model:   mdl,
			Total:   mTotal,
			Buckets: buildBuckets(counts, mTotal),
			Median:  medianSeverity(mRanks),
			Mean:    mMean,
		})
	}
	sort.Slice(byModel, func(i, j int) bool {
		return byModel[i].Mean > byModel[j].Mean
	})

	median := medianSeverity(allRanks)
	rec := fmt.Sprintf("Median severity: %s, mean rank: %.2f/4", median, meanRank)
	if meanRank >= 3.0 {
		rec += "; high severity concentration — prioritize triage"
	}

	return Report{
		TotalFindings:  total,
		Overall:        overall,
		ByModel:        byModel,
		MedianSeverity: median,
		MeanRank:       meanRank,
		Recommendation: rec,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Severity Distribution Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	fmt.Fprintf(&b, "  Total findings:     %d\n", r.TotalFindings)
	fmt.Fprintf(&b, "  Median severity:    %s\n", r.MedianSeverity)
	fmt.Fprintf(&b, "  Mean severity rank: %.2f / 4\n", r.MeanRank)
	fmt.Fprintf(&b, "  %s\n\n", r.Recommendation)

	if len(r.Overall) > 0 {
		b.WriteString("  Overall Distribution:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, sb := range r.Overall {
			bar := strings.Repeat("#", int(sb.Pct*40))
			fmt.Fprintf(&b, "    %-10s %3d (%5.1f%%) %s\n", sb.Severity, sb.Count, sb.Pct*100, bar)
		}
		b.WriteString("\n")
	}

	if len(r.ByModel) > 0 {
		b.WriteString("  By Model:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, md := range r.ByModel {
			fmt.Fprintf(&b, "    %s: %d findings, median=%s, mean=%.2f\n",
				md.Model, md.Total, md.Median, md.Mean)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
