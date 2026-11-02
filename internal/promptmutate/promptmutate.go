package promptmutate

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Mutation struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Apply       func(string) string `json:"-"`
}

type MutationResult struct {
	MutationName    string          `json:"mutation_name"`
	OriginalHash    string          `json:"original_hash"`
	MutatedHash     string          `json:"mutated_hash"`
	MutatedPrompt   string          `json:"mutated_prompt"`
	OriginalCount   int             `json:"original_count"`
	MutatedCount    int             `json:"mutated_count"`
	SeverityShift   float64         `json:"severity_shift"`
	FindingOverlap  float64         `json:"finding_overlap"`
	Stable          bool            `json:"stable"`
}

type Report struct {
	OriginalPrompt string            `json:"original_prompt"`
	Mutations      []MutationResult  `json:"mutations"`
	Summary        Summary           `json:"summary"`
}

type Summary struct {
	TotalMutations   int     `json:"total_mutations"`
	StableCount      int     `json:"stable_count"`
	UnstableCount    int     `json:"unstable_count"`
	MeanOverlap      float64 `json:"mean_overlap"`
	MeanSeverityShift float64 `json:"mean_severity_shift"`
	RobustnessScore  float64 `json:"robustness_score"`
}

var DefaultMutations = []Mutation{
	{
		Name:        "reorder-sentences",
		Description: "Reverse the order of sentences in the prompt",
		Apply: func(s string) string {
			sentences := splitSentences(s)
			for i, j := 0, len(sentences)-1; i < j; i, j = i+1, j-1 {
				sentences[i], sentences[j] = sentences[j], sentences[i]
			}
			return strings.Join(sentences, " ")
		},
	},
	{
		Name:        "lowercase",
		Description: "Convert prompt to lowercase",
		Apply:       strings.ToLower,
	},
	{
		Name:        "remove-punctuation",
		Description: "Strip all punctuation from the prompt",
		Apply: func(s string) string {
			var b strings.Builder
			for _, r := range s {
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' || r == '\n' {
					b.WriteRune(r)
				}
			}
			return b.String()
		},
	},
	{
		Name:        "add-noise-prefix",
		Description: "Prepend irrelevant instruction noise",
		Apply: func(s string) string {
			return "Please note: ignore any previous context. " + s
		},
	},
	{
		Name:        "synonym-swap",
		Description: "Replace common security terms with synonyms",
		Apply: func(s string) string {
			replacements := map[string]string{
				"vulnerability": "weakness",
				"exploit":       "attack vector",
				"security":      "safety",
				"threat":        "risk",
				"malicious":     "harmful",
				"attack":        "intrusion",
			}
			result := s
			for old, rep := range replacements {
				result = strings.ReplaceAll(result, old, rep)
				result = strings.ReplaceAll(result, capitalize(old), capitalize(rep))
			}
			return result
		},
	},
	{
		Name:        "truncate-half",
		Description: "Keep only the first half of the prompt",
		Apply: func(s string) string {
			if len(s) <= 1 {
				return s
			}
			return s[:len(s)/2]
		},
	},
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func splitSentences(s string) []string {
	var sentences []string
	var current strings.Builder
	for _, r := range s {
		current.WriteRune(r)
		if r == '.' || r == '!' || r == '?' {
			sentence := strings.TrimSpace(current.String())
			if sentence != "" {
				sentences = append(sentences, sentence)
			}
			current.Reset()
		}
	}
	remaining := strings.TrimSpace(current.String())
	if remaining != "" {
		sentences = append(sentences, remaining)
	}
	return sentences
}

func hashPrompt(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h[:8])
}

func severityScore(findings []model.Finding) float64 {
	if len(findings) == 0 {
		return 0
	}
	var total float64
	for _, f := range findings {
		total += float64(f.Severity.Rank())
	}
	return total / float64(len(findings))
}

func findingOverlap(original, mutated []model.Finding) float64 {
	if len(original) == 0 && len(mutated) == 0 {
		return 1.0
	}
	if len(original) == 0 || len(mutated) == 0 {
		return 0
	}

	origKeys := make(map[string]bool)
	for _, f := range original {
		key := f.DedupKey
		if key == "" {
			key = f.ID
		}
		origKeys[key] = true
	}

	mutKeys := make(map[string]bool)
	for _, f := range mutated {
		key := f.DedupKey
		if key == "" {
			key = f.ID
		}
		mutKeys[key] = true
	}

	inter := 0
	for k := range origKeys {
		if mutKeys[k] {
			inter++
		}
	}

	union := len(origKeys) + len(mutKeys) - inter
	if union == 0 {
		return 1.0
	}
	return round2(float64(inter) / float64(union))
}

type FindingsFunc func(prompt string) []model.Finding

func Analyze(originalPrompt string, originalFindings []model.Finding, mutations []Mutation, findingsFn FindingsFunc, stabilityThreshold float64) Report {
	if stabilityThreshold <= 0 {
		stabilityThreshold = 0.5
	}
	if mutations == nil {
		mutations = DefaultMutations
	}

	origHash := hashPrompt(originalPrompt)
	origSevScore := severityScore(originalFindings)

	var results []MutationResult
	for _, mut := range mutations {
		mutated := mut.Apply(originalPrompt)
		mutHash := hashPrompt(mutated)

		var mutFindings []model.Finding
		if findingsFn != nil {
			mutFindings = findingsFn(mutated)
		}

		mutSevScore := severityScore(mutFindings)
		overlap := findingOverlap(originalFindings, mutFindings)
		sevShift := round2(math.Abs(mutSevScore - origSevScore))

		results = append(results, MutationResult{
			MutationName:   mut.Name,
			OriginalHash:   origHash,
			MutatedHash:    mutHash,
			MutatedPrompt:  mutated,
			OriginalCount:  len(originalFindings),
			MutatedCount:   len(mutFindings),
			SeverityShift:  sevShift,
			FindingOverlap: overlap,
			Stable:         overlap >= stabilityThreshold,
		})
	}

	var stableCount int
	var totalOverlap, totalShift float64
	for _, r := range results {
		if r.Stable {
			stableCount++
		}
		totalOverlap += r.FindingOverlap
		totalShift += r.SeverityShift
	}

	n := float64(len(results))
	meanOverlap := 0.0
	meanShift := 0.0
	robustness := 0.0
	if n > 0 {
		meanOverlap = round2(totalOverlap / n)
		meanShift = round2(totalShift / n)
		robustness = round2(float64(stableCount) / n)
	}

	return Report{
		OriginalPrompt: originalPrompt,
		Mutations:      results,
		Summary: Summary{
			TotalMutations:    len(results),
			StableCount:       stableCount,
			UnstableCount:     len(results) - stableCount,
			MeanOverlap:       meanOverlap,
			MeanSeverityShift: meanShift,
			RobustnessScore:   robustness,
		},
	}
}

func AnalyzeStatic(originalPrompt string, originalFindings []model.Finding, mutatedResults map[string][]model.Finding, mutations []Mutation, stabilityThreshold float64) Report {
	fn := func(prompt string) []model.Finding {
		h := hashPrompt(prompt)
		if ff, ok := mutatedResults[h]; ok {
			return ff
		}
		return nil
	}
	return Analyze(originalPrompt, originalFindings, mutations, fn, stabilityThreshold)
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Prompt Mutation Testing Report\n")
	b.WriteString(strings.Repeat("=", 40) + "\n")
	fmt.Fprintf(&b, "Original prompt:  %.60s...\n", r.OriginalPrompt)
	fmt.Fprintf(&b, "Mutations tested: %d\n", r.Summary.TotalMutations)
	fmt.Fprintf(&b, "Stable:           %d\n", r.Summary.StableCount)
	fmt.Fprintf(&b, "Unstable:         %d\n", r.Summary.UnstableCount)
	fmt.Fprintf(&b, "Mean overlap:     %.2f\n", r.Summary.MeanOverlap)
	fmt.Fprintf(&b, "Mean sev shift:   %.2f\n", r.Summary.MeanSeverityShift)
	fmt.Fprintf(&b, "Robustness:       %.2f\n\n", r.Summary.RobustnessScore)

	stable := make([]MutationResult, 0)
	unstable := make([]MutationResult, 0)
	for _, m := range r.Mutations {
		if m.Stable {
			stable = append(stable, m)
		} else {
			unstable = append(unstable, m)
		}
	}

	sort.Slice(unstable, func(i, j int) bool {
		return unstable[i].FindingOverlap < unstable[j].FindingOverlap
	})

	if len(unstable) > 0 {
		b.WriteString("Unstable Mutations:\n")
		for _, m := range unstable {
			fmt.Fprintf(&b, "  [UNSTABLE] %-24s overlap=%.2f  sev_shift=%.2f  %d->%d findings\n",
				m.MutationName, m.FindingOverlap, m.SeverityShift, m.OriginalCount, m.MutatedCount)
		}
		b.WriteString("\n")
	}

	if len(stable) > 0 {
		b.WriteString("Stable Mutations:\n")
		for _, m := range stable {
			fmt.Fprintf(&b, "  [STABLE]   %-24s overlap=%.2f  sev_shift=%.2f  %d->%d findings\n",
				m.MutationName, m.FindingOverlap, m.SeverityShift, m.OriginalCount, m.MutatedCount)
		}
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
