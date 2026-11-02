package promptanalyze

import (
	"crypto/sha256"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PromptEffectiveness struct {
	PromptHash    string         `json:"prompt_hash"`
	PromptPreview string         `json:"prompt_preview"`
	FindingCount  int            `json:"finding_count"`
	AvgScore      float64        `json:"avg_score"`
	CheckerHits   map[string]int `json:"checker_hits"`
	Models        []string       `json:"models"`
}

type PromptAnalysis struct {
	Prompts        []PromptEffectiveness `json:"prompts"`
	MostEffective  string                `json:"most_effective"`
	LeastEffective string                `json:"least_effective"`
	TotalPrompts   int                   `json:"total_prompts"`
}

type promptAccum struct {
	hash        string
	preview     string
	count       int
	scoreSum    float64
	checkerHits map[string]int
	modelSet    map[string]bool
}

func Analyze(findings []model.Finding) PromptAnalysis {
	accum := map[string]*promptAccum{}

	for _, f := range findings {
		h := f.PromptHash
		if h == "" {
			h = hashPrompt(f.Summary)
		}
		if accum[h] == nil {
			preview := f.Summary
			if len(preview) > 80 {
				preview = preview[:80]
			}
			accum[h] = &promptAccum{
				hash:        h,
				preview:     preview,
				checkerHits: map[string]int{},
				modelSet:    map[string]bool{},
			}
		}
		a := accum[h]
		a.count++
		a.scoreSum += severityScore(f.Severity)
		if f.Lab != "" {
			a.checkerHits[f.Lab]++
		}
		if f.Model.Model != "" {
			a.modelSet[f.Model.Model] = true
		}
	}

	prompts := make([]PromptEffectiveness, 0, len(accum))
	for _, a := range accum {
		models := make([]string, 0, len(a.modelSet))
		for m := range a.modelSet {
			models = append(models, m)
		}
		sort.Strings(models)

		avg := 0.0
		if a.count > 0 {
			avg = math.Round(a.scoreSum/float64(a.count)*100) / 100
		}

		prompts = append(prompts, PromptEffectiveness{
			PromptHash:    a.hash,
			PromptPreview: a.preview,
			FindingCount:  a.count,
			AvgScore:      avg,
			CheckerHits:   a.checkerHits,
			Models:        models,
		})
	}

	pa := PromptAnalysis{TotalPrompts: len(prompts)}

	// Default sort by finding count descending.
	SortByFindings(prompts)
	pa.Prompts = prompts

	if len(prompts) > 0 {
		pa.MostEffective = prompts[0].PromptHash
		pa.LeastEffective = prompts[len(prompts)-1].PromptHash
	}
	return pa
}

func SortByFindings(prompts []PromptEffectiveness) {
	sort.Slice(prompts, func(i, j int) bool {
		if prompts[i].FindingCount != prompts[j].FindingCount {
			return prompts[i].FindingCount > prompts[j].FindingCount
		}
		return prompts[i].PromptHash < prompts[j].PromptHash
	})
}

func SortByScore(prompts []PromptEffectiveness) {
	sort.Slice(prompts, func(i, j int) bool {
		if prompts[i].AvgScore != prompts[j].AvgScore {
			return prompts[i].AvgScore > prompts[j].AvgScore
		}
		return prompts[i].PromptHash < prompts[j].PromptHash
	})
}

func FormatText(pa PromptAnalysis) string {
	if len(pa.Prompts) == 0 {
		return "No prompts found in run data.\n"
	}

	var b strings.Builder
	b.WriteString("Prompt Effectiveness Analysis\n")
	b.WriteString(strings.Repeat("=", 60) + "\n")
	fmt.Fprintf(&b, "Total prompts: %d\n\n", pa.TotalPrompts)

	fmt.Fprintf(&b, "%-12s %-8s %-8s %-10s %s\n", "Hash", "Finds", "Avg", "Models", "Preview")
	b.WriteString(strings.Repeat("-", 80) + "\n")

	for _, p := range pa.Prompts {
		hash := p.PromptHash
		if len(hash) > 10 {
			hash = hash[:10]
		}
		preview := p.PromptPreview
		if len(preview) > 40 {
			preview = preview[:40] + "..."
		}
		fmt.Fprintf(&b, "%-12s %-8d %-8.2f %-10d %s\n",
			hash, p.FindingCount, p.AvgScore, len(p.Models), preview)
	}

	if pa.MostEffective != "" {
		b.WriteString(fmt.Sprintf("\nMost effective:  %s\n", truncHash(pa.MostEffective)))
		b.WriteString(fmt.Sprintf("Least effective: %s\n", truncHash(pa.LeastEffective)))
	}
	return b.String()
}

func truncHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

func hashPrompt(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

func severityScore(sev model.Severity) float64 {
	switch sev {
	case model.SevCritical:
		return 4.0
	case model.SevHigh:
		return 3.0
	case model.SevMedium:
		return 2.0
	case model.SevLow:
		return 1.0
	default:
		return 0.0
	}
}
