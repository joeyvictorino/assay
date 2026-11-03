package robustness

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PerturbationType string

const (
	PerturbRephrase    PerturbationType = "rephrase"
	PerturbTypo        PerturbationType = "typo"
	PerturbSuffix      PerturbationType = "adversarial-suffix"
	PerturbTranslation PerturbationType = "translation"
	PerturbBaseline    PerturbationType = "baseline"
)

var AllPerturbations = []PerturbationType{
	PerturbBaseline, PerturbRephrase, PerturbTypo, PerturbSuffix, PerturbTranslation,
}

type VariantResult struct {
	PromptHash    string           `json:"prompt_hash"`
	Perturbation  PerturbationType `json:"perturbation"`
	FindingCount  int              `json:"finding_count"`
	CriticalCount int              `json:"critical_count"`
	Classes       []string         `json:"classes,omitempty"`
}

type ModelRobustness struct {
	Model           string          `json:"model"`
	OverallScore    float64         `json:"overall_score"`
	Grade           string          `json:"grade"`
	Variants        []VariantResult `json:"variants"`
	StdDev          float64         `json:"std_dev"`
	MaxDelta        int             `json:"max_delta"`
	MostFragile     string          `json:"most_fragile"`
	MostStable      string          `json:"most_stable"`
	TotalFindings   int             `json:"total_findings"`
	VariantCount    int             `json:"variant_count"`
}

type Report struct {
	Models       []ModelRobustness `json:"models"`
	MostRobust   string            `json:"most_robust"`
	LeastRobust  string            `json:"least_robust"`
	MeanScore    float64           `json:"mean_score"`
}

func classifyPerturbation(f model.Finding) PerturbationType {
	hash := strings.ToLower(f.PromptHash)
	switch {
	case strings.Contains(hash, "rephrase") || strings.Contains(hash, "para"):
		return PerturbRephrase
	case strings.Contains(hash, "typo") || strings.Contains(hash, "misspell"):
		return PerturbTypo
	case strings.Contains(hash, "suffix") || strings.Contains(hash, "adv"):
		return PerturbSuffix
	case strings.Contains(hash, "trans") || strings.Contains(hash, "lang"):
		return PerturbTranslation
	default:
		return PerturbBaseline
	}
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	type key struct {
		model string
		hash  string
	}
	grouped := map[key][]model.Finding{}
	modelNames := map[string]bool{}

	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		k := key{model: mn, hash: f.PromptHash}
		grouped[k] = append(grouped[k], f)
		modelNames[mn] = true
	}

	var models []ModelRobustness
	for mn := range modelNames {
		var variants []VariantResult
		var counts []float64
		total := 0

		perturbCounts := map[PerturbationType]int{}
		for k, fs := range grouped {
			if k.model != mn {
				continue
			}
			pt := classifyPerturbation(fs[0])
			classSet := map[string]bool{}
			crit := 0
			for _, f := range fs {
				classSet[string(f.Class)] = true
				if f.Severity == model.SevCritical {
					crit++
				}
			}
			var classes []string
			for c := range classSet {
				classes = append(classes, c)
			}
			sort.Strings(classes)
			variants = append(variants, VariantResult{
				PromptHash:    k.hash,
				Perturbation:  pt,
				FindingCount:  len(fs),
				CriticalCount: crit,
				Classes:       classes,
			})
			counts = append(counts, float64(len(fs)))
			total += len(fs)
			perturbCounts[pt] += len(fs)
		}

		sort.Slice(variants, func(i, j int) bool {
			return variants[i].FindingCount > variants[j].FindingCount
		})

		sd := stddev(counts)
		maxDelta := 0
		if len(counts) > 1 {
			minC, maxC := counts[0], counts[0]
			for _, c := range counts {
				if c < minC {
					minC = c
				}
				if c > maxC {
					maxC = c
				}
			}
			maxDelta = int(maxC - minC)
		}

		score := 100.0
		score -= sd * 5
		score -= float64(maxDelta) * 2
		if score < 0 {
			score = 0
		}

		mostFragile, mostStable := "", ""
		maxPert, minPert := 0, math.MaxInt
		for pt, cnt := range perturbCounts {
			if cnt > maxPert {
				maxPert = cnt
				mostFragile = string(pt)
			}
			if cnt < minPert {
				minPert = cnt
				mostStable = string(pt)
			}
		}

		models = append(models, ModelRobustness{
			Model:         mn,
			OverallScore:  math.Round(score*10) / 10,
			Grade:         scoreToGrade(score),
			Variants:      variants,
			StdDev:        math.Round(sd*100) / 100,
			MaxDelta:      maxDelta,
			MostFragile:   mostFragile,
			MostStable:    mostStable,
			TotalFindings: total,
			VariantCount:  len(variants),
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].OverallScore > models[j].OverallScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.OverallScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.MostRobust = models[0].Model
		r.LeastRobust = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func stddev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))
	var ss float64
	for _, v := range vals {
		d := v - mean
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(vals)))
}

func scoreToGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Prompt Robustness Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No robustness data available.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models evaluated:   %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:         %.1f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Most robust:        %s\n", r.MostRobust)
	fmt.Fprintf(&b, "  Least robust:       %s\n\n", r.LeastRobust)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  [%s] %.1f/100\n", m.Model, m.Grade, m.OverallScore)
		fmt.Fprintf(&b, "    Variants: %d   Std dev: %.2f   Max delta: %d\n", m.VariantCount, m.StdDev, m.MaxDelta)
		fmt.Fprintf(&b, "    Most fragile: %s   Most stable: %s\n", m.MostFragile, m.MostStable)
		for _, v := range m.Variants {
			fmt.Fprintf(&b, "      [%s] %s: %d findings (%d critical)\n",
				v.Perturbation, v.PromptHash, v.FindingCount, v.CriticalCount)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
