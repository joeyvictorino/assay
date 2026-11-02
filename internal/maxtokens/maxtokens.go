package maxtokens

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Bucket struct {
	MaxTokens      int     `json:"max_tokens"`
	FindingCount   int     `json:"finding_count"`
	AvgLength      float64 `json:"avg_output_length"`
	TruncationRate float64 `json:"truncation_rate"`
	QualityScore   float64 `json:"quality_score"`
	ModelCount     int     `json:"model_count"`
}

type ModelBuckets struct {
	Model          string   `json:"model"`
	Buckets        []Bucket `json:"buckets"`
	OptimalTokens  int      `json:"optimal_max_tokens"`
	WorstTokens    int      `json:"worst_max_tokens"`
}

type Report struct {
	Overall        []Bucket       `json:"overall"`
	ByModel        []ModelBuckets `json:"by_model"`
	TotalFindings  int            `json:"total_findings"`
	OptimalTokens  int            `json:"optimal_max_tokens"`
	Recommendation string         `json:"recommendation"`
}

func severityToNum(sev model.Severity) float64 {
	switch strings.ToLower(string(sev)) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	type bucketKey struct {
		tokens int
		mdl    string
	}

	type accum struct {
		count  int
		sevSum float64
		models map[string]bool
	}

	overallMap := make(map[int]*accum)
	modelMap := make(map[bucketKey]*accum)

	for _, f := range findings {
		tokens := f.MaxTokens
		if tokens <= 0 {
			continue
		}
		sev := severityToNum(f.Severity)

		a, ok := overallMap[tokens]
		if !ok {
			a = &accum{models: make(map[string]bool)}
			overallMap[tokens] = a
		}
		a.count++
		a.sevSum += sev
		a.models[f.Model.Model] = true

		mk := bucketKey{tokens: tokens, mdl: f.Model.Model}
		ma, ok := modelMap[mk]
		if !ok {
			ma = &accum{models: make(map[string]bool)}
			modelMap[mk] = ma
		}
		ma.count++
		ma.sevSum += sev
	}

	if len(overallMap) == 0 {
		return Report{TotalFindings: len(findings), Recommendation: "No max_tokens data available"}
	}

	var overall []Bucket
	bestQuality := -1.0
	optimalTokens := 0
	worstQuality := math.MaxFloat64
	_ = worstQuality

	for tokens, a := range overallMap {
		avgSev := a.sevSum / float64(a.count)
		quality := 1.0 - avgSev/4.0

		bucket := Bucket{
			MaxTokens:      tokens,
			FindingCount:   a.count,
			AvgLength:      0,
			TruncationRate: 0,
			QualityScore:   math.Round(quality*100) / 100,
			ModelCount:     len(a.models),
		}
		overall = append(overall, bucket)

		if quality > bestQuality {
			bestQuality = quality
			optimalTokens = tokens
		}
	}
	sort.Slice(overall, func(i, j int) bool {
		return overall[i].MaxTokens < overall[j].MaxTokens
	})

	mBuckets := make(map[string][]Bucket)
	for mk, a := range modelMap {
		avgSev := a.sevSum / float64(a.count)
		quality := 1.0 - avgSev/4.0

		bucket := Bucket{
			MaxTokens:      mk.tokens,
			FindingCount:   a.count,
			AvgLength:      0,
			TruncationRate: 0,
			QualityScore:   math.Round(quality*100) / 100,
			ModelCount:     1,
		}
		mBuckets[mk.mdl] = append(mBuckets[mk.mdl], bucket)
	}

	var byModel []ModelBuckets
	for mdl, buckets := range mBuckets {
		sort.Slice(buckets, func(i, j int) bool {
			return buckets[i].MaxTokens < buckets[j].MaxTokens
		})
		mb := ModelBuckets{Model: mdl, Buckets: buckets}
		bestQ := -1.0
		worstQ := math.MaxFloat64
		for _, b := range buckets {
			if b.QualityScore > bestQ {
				bestQ = b.QualityScore
				mb.OptimalTokens = b.MaxTokens
			}
			if b.QualityScore < worstQ {
				worstQ = b.QualityScore
				mb.WorstTokens = b.MaxTokens
			}
		}
		byModel = append(byModel, mb)
	}
	sort.Slice(byModel, func(i, j int) bool {
		return byModel[i].Model < byModel[j].Model
	})

	rec := fmt.Sprintf("Optimal max_tokens=%d (quality=%.2f)", optimalTokens, bestQuality)

	return Report{
		Overall:        overall,
		ByModel:        byModel,
		TotalFindings:  len(findings),
		OptimalTokens:  optimalTokens,
		Recommendation: rec,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Max-Tokens Impact Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	fmt.Fprintf(&b, "  Total findings analyzed: %d\n", r.TotalFindings)
	fmt.Fprintf(&b, "  Optimal max_tokens:      %d\n", r.OptimalTokens)
	fmt.Fprintf(&b, "  Recommendation:          %s\n\n", r.Recommendation)

	if len(r.Overall) > 0 {
		b.WriteString("  By Max-Tokens:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, bucket := range r.Overall {
			fmt.Fprintf(&b, "    max_tokens=%d: %d findings, avg_len=%.0f, trunc=%.0f%%, quality=%.2f\n",
				bucket.MaxTokens, bucket.FindingCount, bucket.AvgLength,
				bucket.TruncationRate*100, bucket.QualityScore)
		}
		b.WriteString("\n")
	}

	if len(r.ByModel) > 0 {
		b.WriteString("  By Model:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, mb := range r.ByModel {
			fmt.Fprintf(&b, "    %s (optimal=%d, worst=%d)\n",
				mb.Model, mb.OptimalTokens, mb.WorstTokens)
			for _, bucket := range mb.Buckets {
				fmt.Fprintf(&b, "      max_tokens=%d: %d findings, quality=%.2f\n",
					bucket.MaxTokens, bucket.FindingCount, bucket.QualityScore)
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
