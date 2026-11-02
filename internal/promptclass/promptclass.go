package promptclass

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type Category string

const (
	CatInstruction  Category = "instruction"
	CatExtraction   Category = "extraction"
	CatCreative     Category = "creative"
	CatAnalytical   Category = "analytical"
	CatAdversarial  Category = "adversarial"
	CatSafetyProbe  Category = "safety-probe"
	CatConversation Category = "conversation"
	CatUnknown      Category = "unknown"
)

var AllCategories = []Category{
	CatInstruction, CatExtraction, CatCreative, CatAnalytical,
	CatAdversarial, CatSafetyProbe, CatConversation, CatUnknown,
}

type Classification struct {
	Prompt       string             `json:"prompt"`
	TopCategory  Category           `json:"top_category"`
	Confidence   float64            `json:"confidence"`
	Distribution map[Category]float64 `json:"distribution"`
	MixedIntent  bool               `json:"mixed_intent"`
	Categories   []Category         `json:"categories,omitempty"`
}

type Summary struct {
	Total        int                `json:"total"`
	ByCat        map[Category]int   `json:"by_category"`
	MixedCount   int                `json:"mixed_count"`
	AvgConfidence float64           `json:"avg_confidence"`
}

var categoryPatterns = map[Category][]string{
	CatInstruction: {
		"implement", "create", "build", "write", "add", "modify",
		"update", "fix", "install", "configure", "set up", "deploy",
		"generate", "define", "make", "construct", "refactor",
		"convert", "transform", "migrate", "change", "remove",
		"delete", "rename", "move", "copy", "replace", "insert",
	},
	CatExtraction: {
		"extract", "find", "list", "show", "get", "fetch",
		"retrieve", "parse", "read", "search", "lookup", "query",
		"identify", "locate", "return", "output", "display",
		"enumerate", "scan", "detect", "discover", "collect",
	},
	CatCreative: {
		"imagine", "story", "poem", "creative", "fiction",
		"narrative", "compose", "invent", "dream", "fantasy",
		"roleplay", "pretend", "scenario", "brainstorm",
		"metaphor", "analogy", "novel", "character",
	},
	CatAnalytical: {
		"analyze", "compare", "evaluate", "assess", "review",
		"examine", "investigate", "study", "measure", "benchmark",
		"profile", "audit", "inspect", "diagnose", "trace",
		"explain", "summarize", "classify", "categorize", "rank",
	},
	CatAdversarial: {
		"ignore previous", "disregard", "bypass", "override",
		"jailbreak", "pretend you are", "act as", "you are now",
		"forget your instructions", "new persona", "dan mode",
		"evil mode", "opposite day", "system prompt",
		"reveal your", "show me your prompt", "inject",
		"encoded", "base64", "rot13", "obfuscate",
	},
	CatSafetyProbe: {
		"harmful", "dangerous", "illegal", "weapon", "exploit",
		"vulnerability", "attack", "hack", "malware", "virus",
		"phishing", "social engineer", "steal", "fraud",
		"bomb", "poison", "drug", "violence", "threat",
		"abuse", "self-harm", "suicide", "kill", "destroy",
	},
	CatConversation: {
		"hello", "hi", "hey", "how are you", "what do you think",
		"tell me about", "can you", "what is", "who is",
		"thank you", "thanks", "please", "help me understand",
		"opinion", "thoughts on", "chat", "discuss",
	},
}

func Classify(prompt string) Classification {
	lower := strings.ToLower(prompt)
	words := strings.Fields(lower)

	scores := make(map[Category]float64)
	for cat, patterns := range categoryPatterns {
		score := matchScore(lower, words, patterns)
		if score > 0 {
			scores[cat] = score
		}
	}

	if len(scores) == 0 {
		scores[CatUnknown] = 1.0
	}

	dist := normalize(scores)

	var topCat Category
	var topScore float64
	for cat, score := range dist {
		if score > topScore {
			topScore = score
			topCat = cat
		}
	}

	var mixed []Category
	for cat, score := range dist {
		if score >= 0.2 {
			mixed = append(mixed, cat)
		}
	}
	sort.Slice(mixed, func(i, j int) bool {
		return dist[mixed[i]] > dist[mixed[j]]
	})

	return Classification{
		Prompt:       truncate(prompt, 200),
		TopCategory:  topCat,
		Confidence:   topScore,
		Distribution: dist,
		MixedIntent:  len(mixed) > 1,
		Categories:   mixed,
	}
}

func ClassifyAll(prompts []string) []Classification {
	out := make([]Classification, len(prompts))
	for i, p := range prompts {
		out[i] = Classify(p)
	}
	return out
}

func Summarize(results []Classification) Summary {
	s := Summary{
		Total: len(results),
		ByCat: make(map[Category]int),
	}
	var totalConf float64
	for _, r := range results {
		s.ByCat[r.TopCategory]++
		if r.MixedIntent {
			s.MixedCount++
		}
		totalConf += r.Confidence
	}
	if s.Total > 0 {
		s.AvgConfidence = totalConf / float64(s.Total)
	}
	return s
}

func FormatText(results []Classification) string {
	var sb strings.Builder
	sb.WriteString("Prompt Classification Results\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")

	for i, r := range results {
		fmt.Fprintf(&sb, "%d. [%s] (confidence: %.3f)", i+1, r.TopCategory, r.Confidence)
		if r.MixedIntent {
			cats := make([]string, len(r.Categories))
			for j, c := range r.Categories {
				cats[j] = string(c)
			}
			fmt.Fprintf(&sb, " mixed: %s", strings.Join(cats, "+"))
		}
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "   %s\n\n", r.Prompt)
	}
	return sb.String()
}

func FormatSummaryText(s Summary) string {
	var sb strings.Builder
	sb.WriteString("Prompt Classification Summary\n")
	sb.WriteString(strings.Repeat("=", 60) + "\n\n")
	fmt.Fprintf(&sb, "  Total Prompts:     %d\n", s.Total)
	fmt.Fprintf(&sb, "  Mixed Intent:      %d\n", s.MixedCount)
	fmt.Fprintf(&sb, "  Avg Confidence:    %.3f\n\n", s.AvgConfidence)

	type catCount struct {
		cat   Category
		count int
	}
	var sorted []catCount
	for c, n := range s.ByCat {
		sorted = append(sorted, catCount{c, n})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].count > sorted[j].count })

	sb.WriteString("  Category Distribution:\n")
	for _, cc := range sorted {
		pct := 0.0
		if s.Total > 0 {
			pct = float64(cc.count) / float64(s.Total) * 100
		}
		fmt.Fprintf(&sb, "    %-16s %4d  (%5.1f%%)\n", cc.cat, cc.count, pct)
	}
	sb.WriteString("\n")
	return sb.String()
}

func matchScore(lower string, words []string, patterns []string) float64 {
	var score float64
	for _, p := range patterns {
		if strings.Contains(p, " ") {
			if strings.Contains(lower, p) {
				score += 2.0
			}
		} else {
			for _, w := range words {
				if w == p || strings.HasPrefix(w, p) {
					score += 1.0
					break
				}
			}
		}
	}
	return score
}

func normalize(scores map[Category]float64) map[Category]float64 {
	var total float64
	for _, v := range scores {
		total += v
	}
	dist := make(map[Category]float64, len(scores))
	if total == 0 {
		return dist
	}
	for k, v := range scores {
		dist[k] = math.Round(v/total*1000) / 1000
	}
	return dist
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
