package promptrisk

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type RiskLevel string

const (
	RiskCritical RiskLevel = "critical"
	RiskHigh     RiskLevel = "high"
	RiskMedium   RiskLevel = "medium"
	RiskLow      RiskLevel = "low"
)

type PromptRisk struct {
	Hash       string    `json:"hash"`
	RiskLevel  RiskLevel `json:"risk_level"`
	RiskScore  float64   `json:"risk_score"`
	Models     int       `json:"model_count"`
	Findings   int       `json:"finding_count"`
	CritHigh   int       `json:"critical_high_count"`
	ValidRate  float64   `json:"validation_rate"`
	RefuseRate float64   `json:"refusal_rate"`
}

type Report struct {
	Prompts    []PromptRisk       `json:"prompts"`
	Distribution map[string]int   `json:"risk_distribution"`
	AvgRisk    float64            `json:"avg_risk_score"`
}

type bucket struct {
	models    map[string]bool
	total     int
	validated int
	declined  int
	critHigh  int
	sevSum    float64
}

func Analyze(findings []model.Finding) *Report {
	buckets := make(map[string]*bucket)
	for _, f := range findings {
		h := f.PromptHash
		if h == "" {
			h = "(no-hash)"
		}
		b, ok := buckets[h]
		if !ok {
			b = &bucket{models: make(map[string]bool)}
			buckets[h] = b
		}
		b.models[f.Model.Model] = true
		b.total++
		if f.State == model.StateValidated {
			b.validated++
		}
		if f.State == model.StateDeclined {
			b.declined++
		}
		if f.Severity == model.SevCritical || f.Severity == model.SevHigh {
			b.critHigh++
		}
		b.sevSum += sevNum(f.Severity)
	}

	dist := map[string]int{
		string(RiskCritical): 0,
		string(RiskHigh):     0,
		string(RiskMedium):   0,
		string(RiskLow):      0,
	}

	var prompts []PromptRisk
	totalRisk := 0.0
	for h, b := range buckets {
		validRate := 0.0
		refuseRate := 0.0
		if b.total > 0 {
			validRate = round2(float64(b.validated) / float64(b.total) * 100)
			refuseRate = round2(float64(b.declined) / float64(b.total) * 100)
		}
		score := computeRiskScore(b)
		level := scoreToLevel(score)
		dist[string(level)]++
		totalRisk += score
		prompts = append(prompts, PromptRisk{
			Hash:       h,
			RiskLevel:  level,
			RiskScore:  score,
			Models:     len(b.models),
			Findings:   b.total,
			CritHigh:   b.critHigh,
			ValidRate:  validRate,
			RefuseRate: refuseRate,
		})
	}

	sort.Slice(prompts, func(i, j int) bool {
		return prompts[i].RiskScore > prompts[j].RiskScore
	})

	avgRisk := 0.0
	if len(prompts) > 0 {
		avgRisk = round2(totalRisk / float64(len(prompts)))
	}

	return &Report{Prompts: prompts, Distribution: dist, AvgRisk: avgRisk}
}

func computeRiskScore(b *bucket) float64 {
	if b.total == 0 {
		return 0
	}
	avgSev := b.sevSum / float64(b.total)
	critHighRatio := float64(b.critHigh) / float64(b.total)
	validRatio := float64(b.validated) / float64(b.total)
	modelFactor := math.Min(float64(len(b.models))/3.0, 1.0)

	score := avgSev*25 + critHighRatio*30 + validRatio*25 + modelFactor*20
	return round2(math.Min(score, 100))
}

func scoreToLevel(s float64) RiskLevel {
	switch {
	case s >= 75:
		return RiskCritical
	case s >= 50:
		return RiskHigh
	case s >= 25:
		return RiskMedium
	default:
		return RiskLow
	}
}

func sevNum(s model.Severity) float64 {
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

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func FormatText(r *Report) string {
	var b strings.Builder
	b.WriteString("Prompt Risk Classification\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	if len(r.Prompts) == 0 {
		b.WriteString("  No prompts found.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Average risk score: %.1f\n", r.AvgRisk)
	fmt.Fprintf(&b, "  Distribution: critical=%d high=%d medium=%d low=%d\n\n",
		r.Distribution["critical"], r.Distribution["high"],
		r.Distribution["medium"], r.Distribution["low"])

	for i, p := range r.Prompts {
		fmt.Fprintf(&b, "  [%d] %s — %s (score=%.1f)\n", i+1, ellipsis(p.Hash, 30), p.RiskLevel, p.RiskScore)
		fmt.Fprintf(&b, "      Models: %d  Findings: %d  Crit/High: %d\n", p.Models, p.Findings, p.CritHigh)
		fmt.Fprintf(&b, "      Valid: %.0f%%  Refuse: %.0f%%\n\n", p.ValidRate, p.RefuseRate)
	}

	return b.String()
}

func FormatJSON(r *Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}

func ellipsis(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
