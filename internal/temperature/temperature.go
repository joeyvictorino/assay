package temperature

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type TempBucket struct {
	Temperature   float64 `json:"temperature"`
	FindingCount  int     `json:"finding_count"`
	SafetyScore   float64 `json:"safety_score"`
	AvgSeverity   float64 `json:"avg_severity"`
	ModelCount    int     `json:"model_count"`
}

type ModelTemp struct {
	Model       string       `json:"model"`
	Buckets     []TempBucket `json:"buckets"`
	SafestTemp  float64      `json:"safest_temperature"`
	RiskiestTemp float64     `json:"riskiest_temperature"`
}

type Report struct {
	Overall       []TempBucket `json:"overall"`
	ByModel       []ModelTemp  `json:"by_model"`
	TotalFindings int          `json:"total_findings"`
	SafestTemp    float64      `json:"safest_temperature"`
	DegradationAt float64      `json:"safety_degrades_at"`
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

func roundTemp(t float64) float64 {
	return math.Round(t*10) / 10
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	type bucketKey struct {
		temp  float64
		model string
	}

	type accum struct {
		count    int
		sevSum   float64
		models   map[string]bool
	}

	overallMap := make(map[float64]*accum)
	modelMap := make(map[bucketKey]*accum)

	for _, f := range findings {
		temp := roundTemp(f.Temperature)
		sev := severityToNum(f.Severity)

		a, ok := overallMap[temp]
		if !ok {
			a = &accum{models: make(map[string]bool)}
			overallMap[temp] = a
		}
		a.count++
		a.sevSum += sev
		a.models[f.Model.Model] = true

		mk := bucketKey{temp: temp, model: f.Model.Model}
		ma, ok := modelMap[mk]
		if !ok {
			ma = &accum{models: make(map[string]bool)}
			modelMap[mk] = ma
		}
		ma.count++
		ma.sevSum += sev
	}

	var overall []TempBucket
	var safestTemp float64
	bestSafety := -1.0
	var degradeTemp float64
	worstSafety := math.MaxFloat64

	for temp, a := range overallMap {
		avgSev := a.sevSum / float64(a.count)
		safety := 1.0 - (avgSev / 4.0)
		bucket := TempBucket{
			Temperature:  temp,
			FindingCount: a.count,
			SafetyScore:  math.Round(safety*100) / 100,
			AvgSeverity:  math.Round(avgSev*100) / 100,
			ModelCount:   len(a.models),
		}
		overall = append(overall, bucket)

		if safety > bestSafety {
			bestSafety = safety
			safestTemp = temp
		}
		if safety < worstSafety {
			worstSafety = safety
			degradeTemp = temp
		}
	}
	sort.Slice(overall, func(i, j int) bool {
		return overall[i].Temperature < overall[j].Temperature
	})

	modelBuckets := make(map[string][]TempBucket)
	for mk, a := range modelMap {
		avgSev := a.sevSum / float64(a.count)
		safety := 1.0 - (avgSev / 4.0)
		bucket := TempBucket{
			Temperature:  mk.temp,
			FindingCount: a.count,
			SafetyScore:  math.Round(safety*100) / 100,
			AvgSeverity:  math.Round(avgSev*100) / 100,
			ModelCount:   1,
		}
		modelBuckets[mk.model] = append(modelBuckets[mk.model], bucket)
	}

	var byModel []ModelTemp
	for mdl, buckets := range modelBuckets {
		sort.Slice(buckets, func(i, j int) bool {
			return buckets[i].Temperature < buckets[j].Temperature
		})
		mt := ModelTemp{Model: mdl, Buckets: buckets}
		best := -1.0
		worst := math.MaxFloat64
		for _, b := range buckets {
			if b.SafetyScore > best {
				best = b.SafetyScore
				mt.SafestTemp = b.Temperature
			}
			if b.SafetyScore < worst {
				worst = b.SafetyScore
				mt.RiskiestTemp = b.Temperature
			}
		}
		byModel = append(byModel, mt)
	}
	sort.Slice(byModel, func(i, j int) bool {
		return byModel[i].Model < byModel[j].Model
	})

	return Report{
		Overall:       overall,
		ByModel:       byModel,
		TotalFindings: len(findings),
		SafestTemp:    safestTemp,
		DegradationAt: degradeTemp,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Temperature Sensitivity Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	fmt.Fprintf(&b, "  Total findings analyzed: %d\n", r.TotalFindings)
	fmt.Fprintf(&b, "  Safest temperature:      %.1f\n", r.SafestTemp)
	fmt.Fprintf(&b, "  Safety degrades at:      %.1f\n\n", r.DegradationAt)

	if len(r.Overall) > 0 {
		b.WriteString("  By Temperature:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, bucket := range r.Overall {
			fmt.Fprintf(&b, "    temp=%.1f: %d findings, safety=%.2f, avg_severity=%.2f\n",
				bucket.Temperature, bucket.FindingCount, bucket.SafetyScore, bucket.AvgSeverity)
		}
		b.WriteString("\n")
	}

	if len(r.ByModel) > 0 {
		b.WriteString("  By Model:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, mt := range r.ByModel {
			fmt.Fprintf(&b, "    %s (safest=%.1f, riskiest=%.1f)\n",
				mt.Model, mt.SafestTemp, mt.RiskiestTemp)
			for _, bucket := range mt.Buckets {
				fmt.Fprintf(&b, "      temp=%.1f: %d findings, safety=%.2f\n",
					bucket.Temperature, bucket.FindingCount, bucket.SafetyScore)
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
