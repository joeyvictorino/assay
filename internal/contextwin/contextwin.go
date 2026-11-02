package contextwin

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var modelContextWindows = map[string]int{
	"claude-3-opus":          200000,
	"claude-3-sonnet":        200000,
	"claude-3-haiku":         200000,
	"claude-3.5-sonnet":      200000,
	"claude-3.5-haiku":       200000,
	"claude-opus-4":          200000,
	"claude-sonnet-4":        200000,
	"gpt-4":                  128000,
	"gpt-4-turbo":            128000,
	"gpt-4o":                 128000,
	"gpt-3.5-turbo":          16385,
	"gemini-1.5-pro":         1000000,
	"gemini-1.5-flash":       1000000,
}

const defaultContextWindow = 128000
const wordsToTokenRatio = 1.3

type RunAnalysis struct {
	Model           string  `json:"model"`
	Checker         string  `json:"checker"`
	ContextWindow   int     `json:"context_window"`
	EstPromptTokens int     `json:"est_prompt_tokens"`
	EstRespTokens   int     `json:"est_response_tokens"`
	TotalTokens     int     `json:"total_tokens"`
	Utilization     float64 `json:"utilization_pct"`
	WasteRatio      float64 `json:"waste_ratio"`
	RepeatRatio     float64 `json:"repeat_ratio"`
}

type Suggestion struct {
	Type    string `json:"type"`
	Detail  string `json:"detail"`
	Impact  string `json:"impact"`
}

type Summary struct {
	TotalFindings    int     `json:"total_findings"`
	AvgUtilization   float64 `json:"avg_utilization_pct"`
	AvgWasteRatio    float64 `json:"avg_waste_ratio"`
	HighWasteCount   int     `json:"high_waste_count"`
	Suggestions      int     `json:"suggestions"`
}

type Report struct {
	Runs        []RunAnalysis `json:"runs"`
	Suggestions []Suggestion  `json:"suggestions"`
	Summary     Summary       `json:"summary"`
}

func Analyze(findings []model.Finding) Report {
	modelRuns := map[string]*runAgg{}
	for _, f := range findings {
		mn := f.Model.Model
		key := mn + "|" + f.Lab
		agg, ok := modelRuns[key]
		if !ok {
			agg = &runAgg{model: mn, checker: f.Lab, texts: map[string]int{}}
			modelRuns[key] = agg
		}
		words := countWords(f.Summary)
		for _, ev := range f.Evidence {
			words += countWords(ev.Kind + " " + ev.Marker)
		}
		agg.totalWords += words
		agg.count++

		for _, chunk := range extractChunks(f.Summary) {
			agg.texts[chunk]++
		}
		for _, ev := range f.Evidence {
			for _, chunk := range extractChunks(ev.Kind + " " + ev.Marker) {
				agg.texts[chunk]++
			}
		}
	}

	var runs []RunAnalysis
	var suggestions []Suggestion
	var utilSum, wasteSum float64
	highWaste := 0

	for _, agg := range sortedAggs(modelRuns) {
		ctxWin := contextWindowFor(agg.model)
		promptTok := int(float64(agg.totalWords) * wordsToTokenRatio)
		respTok := promptTok / 3
		total := promptTok + respTok
		util := 0.0
		if ctxWin > 0 {
			util = math.Min(100.0, float64(total)/float64(ctxWin)*100.0)
		}

		repeats := 0
		totalChunks := 0
		for _, cnt := range agg.texts {
			totalChunks += cnt
			if cnt > 1 {
				repeats += cnt - 1
			}
		}
		repeatRatio := 0.0
		if totalChunks > 0 {
			repeatRatio = float64(repeats) / float64(totalChunks) * 100.0
		}

		waste := repeatRatio / 100.0
		if waste > 0.3 {
			highWaste++
		}

		runs = append(runs, RunAnalysis{
			Model:           agg.model,
			Checker:         agg.checker,
			ContextWindow:   ctxWin,
			EstPromptTokens: promptTok,
			EstRespTokens:   respTok,
			TotalTokens:     total,
			Utilization:     math.Round(util*10) / 10,
			WasteRatio:      math.Round(waste*1000) / 1000,
			RepeatRatio:     math.Round(repeatRatio*10) / 10,
		})
		utilSum += util
		wasteSum += waste
	}

	if highWaste > 0 {
		suggestions = append(suggestions, Suggestion{
			Type:   "deduplication",
			Detail: fmt.Sprintf("%d model/checker pairs have >30%% repeated content", highWaste),
			Impact: "reduce token cost by removing duplicate context",
		})
	}

	avgUtil := 0.0
	avgWaste := 0.0
	if len(runs) > 0 {
		avgUtil = utilSum / float64(len(runs))
		avgWaste = wasteSum / float64(len(runs))
	}

	lowUtil := 0
	for _, r := range runs {
		if r.Utilization < 10 {
			lowUtil++
		}
	}
	if lowUtil > 0 {
		suggestions = append(suggestions, Suggestion{
			Type:   "batching",
			Detail: fmt.Sprintf("%d model/checker pairs use <10%% of context window", lowUtil),
			Impact: "batch prompts to improve throughput and reduce per-call overhead",
		})
	}

	return Report{
		Runs:        runs,
		Suggestions: suggestions,
		Summary: Summary{
			TotalFindings:  len(findings),
			AvgUtilization: math.Round(avgUtil*10) / 10,
			AvgWasteRatio:  math.Round(avgWaste*1000) / 1000,
			HighWasteCount: highWaste,
			Suggestions:    len(suggestions),
		},
	}
}

type runAgg struct {
	model      string
	checker    string
	totalWords int
	count      int
	texts      map[string]int
}

func contextWindowFor(name string) int {
	for prefix, win := range modelContextWindows {
		if strings.HasPrefix(name, prefix) {
			return win
		}
	}
	return defaultContextWindow
}

func countWords(s string) int {
	return len(strings.Fields(s))
}

func extractChunks(text string) []string {
	words := strings.Fields(text)
	if len(words) < 5 {
		return nil
	}
	var chunks []string
	for i := 0; i <= len(words)-5; i++ {
		chunks = append(chunks, strings.Join(words[i:i+5], " "))
	}
	return chunks
}

func sortedAggs(m map[string]*runAgg) []*runAgg {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	result := make([]*runAgg, len(keys))
	for i, k := range keys {
		result[i] = m[k]
	}
	return result
}

func FormatText(r Report) string {
	var sb strings.Builder
	sb.WriteString("Context Window Utilization Analysis\n")
	sb.WriteString(strings.Repeat("=", 62) + "\n\n")
	sb.WriteString(fmt.Sprintf("  Total findings:        %d\n", r.Summary.TotalFindings))
	sb.WriteString(fmt.Sprintf("  Avg utilization:       %.1f%%\n", r.Summary.AvgUtilization))
	sb.WriteString(fmt.Sprintf("  Avg waste ratio:       %.3f\n", r.Summary.AvgWasteRatio))
	sb.WriteString(fmt.Sprintf("  High waste pairs:      %d\n", r.Summary.HighWasteCount))
	sb.WriteString(fmt.Sprintf("  Optimization hints:    %d\n\n", r.Summary.Suggestions))

	if len(r.Runs) > 0 {
		sb.WriteString("  Per-model/checker:\n")
		for _, ra := range r.Runs {
			sb.WriteString(fmt.Sprintf("    %s / %s: %.1f%% util, %.1f%% repeat (%d est tokens / %d window)\n",
				ra.Model, ra.Checker, ra.Utilization, ra.RepeatRatio, ra.TotalTokens, ra.ContextWindow))
		}
		sb.WriteString("\n")
	}

	if len(r.Suggestions) > 0 {
		sb.WriteString("  Suggestions:\n")
		for _, s := range r.Suggestions {
			sb.WriteString(fmt.Sprintf("    [%s] %s — %s\n", s.Type, s.Detail, s.Impact))
		}
	}
	return sb.String()
}

func FormatJSON(r Report) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b) + "\n"
}
