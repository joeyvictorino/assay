package fingerprint

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type StyleMetrics struct {
	AvgSummaryLen float64            `json:"avg_summary_len"`
	SevDist       map[string]float64 `json:"severity_distribution"`
	ClassDist     map[string]float64 `json:"class_distribution"`
	ValidRate     float64            `json:"validation_rate"`
	RefuseRate    float64            `json:"refusal_rate"`
	AvgEvidence   float64            `json:"avg_evidence_count"`
}

type ModelFingerprint struct {
	Model   string       `json:"model"`
	Count   int          `json:"count"`
	Style   StyleMetrics `json:"style"`
	Entropy float64      `json:"entropy"`
}

type CompareResult struct {
	ModelA     string  `json:"model_a"`
	ModelB     string  `json:"model_b"`
	Similarity float64 `json:"similarity"`
	Divergence float64 `json:"divergence"`
}

type Report struct {
	Fingerprints []ModelFingerprint `json:"fingerprints"`
	Comparisons  []CompareResult   `json:"comparisons,omitempty"`
}

type bucket struct {
	summaryLens []int
	sevCounts   map[model.Severity]int
	classCounts map[model.Class]int
	validated   int
	declined    int
	evidCount   int
	total       int
}

func Analyze(findings []model.Finding) *Report {
	buckets := make(map[string]*bucket)
	for _, f := range findings {
		m := f.Model.Model
		b, ok := buckets[m]
		if !ok {
			b = &bucket{
				sevCounts:   make(map[model.Severity]int),
				classCounts: make(map[model.Class]int),
			}
			buckets[m] = b
		}
		b.total++
		b.summaryLens = append(b.summaryLens, len(f.Summary))
		b.sevCounts[f.Severity]++
		b.classCounts[f.Class]++
		if f.State == model.StateValidated {
			b.validated++
		}
		if f.State == model.StateDeclined {
			b.declined++
		}
		b.evidCount += len(f.Evidence)
	}

	var fps []ModelFingerprint
	for m, b := range buckets {
		style := computeStyle(b)
		fps = append(fps, ModelFingerprint{
			Model:   m,
			Count:   b.total,
			Style:   style,
			Entropy: shannonEntropy(b.sevCounts),
		})
	}
	sort.Slice(fps, func(i, j int) bool {
		return fps[i].Model < fps[j].Model
	})

	var comps []CompareResult
	for i := 0; i < len(fps); i++ {
		for j := i + 1; j < len(fps); j++ {
			sim := cosineSimilarity(fps[i].Style.SevDist, fps[j].Style.SevDist)
			comps = append(comps, CompareResult{
				ModelA:     fps[i].Model,
				ModelB:     fps[j].Model,
				Similarity: round2(sim),
				Divergence: round2(1 - sim),
			})
		}
	}

	return &Report{Fingerprints: fps, Comparisons: comps}
}

func computeStyle(b *bucket) StyleMetrics {
	avgLen := 0.0
	if len(b.summaryLens) > 0 {
		sum := 0
		for _, l := range b.summaryLens {
			sum += l
		}
		avgLen = round2(float64(sum) / float64(len(b.summaryLens)))
	}

	sevDist := make(map[string]float64)
	for s, c := range b.sevCounts {
		sevDist[string(s)] = round2(float64(c) / float64(b.total))
	}

	classDist := make(map[string]float64)
	for c, n := range b.classCounts {
		classDist[string(c)] = round2(float64(n) / float64(b.total))
	}

	validRate := 0.0
	refuseRate := 0.0
	avgEvid := 0.0
	if b.total > 0 {
		validRate = round2(float64(b.validated) / float64(b.total))
		refuseRate = round2(float64(b.declined) / float64(b.total))
		avgEvid = round2(float64(b.evidCount) / float64(b.total))
	}

	return StyleMetrics{
		AvgSummaryLen: avgLen,
		SevDist:       sevDist,
		ClassDist:     classDist,
		ValidRate:     validRate,
		RefuseRate:    refuseRate,
		AvgEvidence:   avgEvid,
	}
}

func shannonEntropy[K comparable](counts map[K]int) float64 {
	total := 0
	for _, c := range counts {
		total += c
	}
	if total == 0 {
		return 0
	}
	entropy := 0.0
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := float64(c) / float64(total)
		entropy -= p * math.Log2(p)
	}
	return round2(entropy)
}

func cosineSimilarity(a, b map[string]float64) float64 {
	keys := make(map[string]bool)
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	dotProduct := 0.0
	magA := 0.0
	magB := 0.0
	for k := range keys {
		va := a[k]
		vb := b[k]
		dotProduct += va * vb
		magA += va * va
		magB += vb * vb
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(magA) * math.Sqrt(magB))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func FormatText(r *Report) string {
	var b strings.Builder
	b.WriteString("Model Fingerprint Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	if len(r.Fingerprints) == 0 {
		b.WriteString("  No model data available.\n")
		return b.String()
	}

	for i, fp := range r.Fingerprints {
		fmt.Fprintf(&b, "  [%d] %s (%d findings)\n", i+1, fp.Model, fp.Count)
		fmt.Fprintf(&b, "      Avg summary length: %.0f chars\n", fp.Style.AvgSummaryLen)
		fmt.Fprintf(&b, "      Validation rate:    %.0f%%\n", fp.Style.ValidRate*100)
		fmt.Fprintf(&b, "      Refusal rate:       %.0f%%\n", fp.Style.RefuseRate*100)
		fmt.Fprintf(&b, "      Avg evidence:       %.1f\n", fp.Style.AvgEvidence)
		fmt.Fprintf(&b, "      Severity entropy:   %.2f bits\n", fp.Entropy)
		b.WriteString("\n")
	}

	if len(r.Comparisons) > 0 {
		b.WriteString("  Pairwise Comparisons\n")
		b.WriteString("  " + strings.Repeat("-", 50) + "\n")
		for _, c := range r.Comparisons {
			fmt.Fprintf(&b, "    %s vs %s: similarity=%.0f%% divergence=%.0f%%\n",
				c.ModelA, c.ModelB, c.Similarity*100, c.Divergence*100)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r *Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
