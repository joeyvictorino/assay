package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

func init() {
	Register("findings-prioritize", "rank findings by risk priority using severity, frequency, and exploitability", runFindingsPrioritize)
}

type prioritizedFinding struct {
	Index        int            `json:"index"`
	Score        float64        `json:"score"`
	Severity     string         `json:"severity"`
	Checker      string         `json:"checker"`
	Message      string         `json:"message"`
	Class        string         `json:"class,omitempty"`
	File         string         `json:"file,omitempty"`
	Exploitable  bool           `json:"exploitable,omitempty"`
	Raw          map[string]any `json:"-"`
}

type prioritizeResult struct {
	Total        int                  `json:"total"`
	Prioritized  []prioritizedFinding `json:"prioritized"`
	TopN         int                  `json:"top_n"`
	ScoreWeights map[string]float64   `json:"score_weights"`
}

func runFindingsPrioritize(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay findings-prioritize", stderr)
	input := fs.String("input", "", "path to findings JSON file (required)")
	topN := fs.Int("top", 10, "show top N prioritized findings")
	format := fs.String("format", "text", "output format: text or json")
	sevWeight := fs.Float64("severity-weight", 0.5, "weight for severity score (0-1)")
	freqWeight := fs.Float64("frequency-weight", 0.3, "weight for frequency score (0-1)")
	exploitWeight := fs.Float64("exploit-weight", 0.2, "weight for exploitability score (0-1)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay findings-prioritize: --input is required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintf(stderr, "assay findings-prioritize: %v\n", err)
		return ExitError
	}

	var findings []map[string]any
	if err := json.Unmarshal(data, &findings); err != nil {
		fmt.Fprintf(stderr, "assay findings-prioritize: invalid JSON: %v\n", err)
		return ExitError
	}

	checkerFreq := make(map[string]int)
	for _, f := range findings {
		chk, _ := f["checker"].(string)
		checkerFreq[chk]++
	}

	maxFreq := 1
	for _, c := range checkerFreq {
		if c > maxFreq {
			maxFreq = c
		}
	}

	weights := map[string]float64{
		"severity":      *sevWeight,
		"frequency":     *freqWeight,
		"exploitability": *exploitWeight,
	}

	var prioritized []prioritizedFinding
	for i, f := range findings {
		sev, _ := f["severity"].(string)
		chk, _ := f["checker"].(string)
		msg, _ := f["message"].(string)
		cls, _ := f["class"].(string)
		file, _ := f["file"].(string)

		sevScore := severityScore(sev)
		freqScore := float64(checkerFreq[chk]) / float64(maxFreq)
		exploitScore := exploitabilityScore(f)

		score := sevScore*weights["severity"] +
			freqScore*weights["frequency"] +
			exploitScore*weights["exploitability"]

		prioritized = append(prioritized, prioritizedFinding{
			Index:       i,
			Score:       score,
			Severity:    sev,
			Checker:     chk,
			Message:     msg,
			Class:       cls,
			File:        file,
			Exploitable: exploitScore > 0.5,
			Raw:         f,
		})
	}

	sort.Slice(prioritized, func(i, j int) bool {
		return prioritized[i].Score > prioritized[j].Score
	})

	if *topN > 0 && *topN < len(prioritized) {
		prioritized = prioritized[:*topN]
	}

	result := prioritizeResult{
		Total:        len(findings),
		Prioritized:  prioritized,
		TopN:         *topN,
		ScoreWeights: weights,
	}

	if *format == "json" {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.Encode(result)
	} else {
		fmt.Fprintf(stdout, "Findings Prioritization (top %d of %d)\n\n", len(prioritized), len(findings))
		fmt.Fprintf(stdout, "%-5s %-8s %-7s %-20s %s\n", "Rank", "Score", "Sev", "Checker", "Message")
		fmt.Fprintf(stdout, "%s\n", strings.Repeat("-", 72))
		for rank, pf := range prioritized {
			msg := pf.Message
			if len(msg) > 40 {
				msg = msg[:37] + "..."
			}
			fmt.Fprintf(stdout, "%-5d %-8.3f %-7s %-20s %s\n", rank+1, pf.Score, pf.Severity, pf.Checker, msg)
		}
	}
	return ExitPass
}

func severityScore(sev string) float64 {
	switch strings.ToLower(sev) {
	case "critical":
		return 1.0
	case "high":
		return 0.8
	case "medium":
		return 0.5
	case "low":
		return 0.2
	case "info", "informational":
		return 0.1
	default:
		return 0.3
	}
}

func exploitabilityScore(f map[string]any) float64 {
	score := 0.0
	if v, ok := f["exploitable"]; ok {
		if b, ok := v.(bool); ok && b {
			return 1.0
		}
	}
	if v, _ := f["class"].(string); v != "" {
		lower := strings.ToLower(v)
		if strings.Contains(lower, "injection") || strings.Contains(lower, "rce") || strings.Contains(lower, "command") {
			score += 0.4
		}
		if strings.Contains(lower, "auth") || strings.Contains(lower, "credential") {
			score += 0.3
		}
	}
	sev, _ := f["severity"].(string)
	if strings.EqualFold(sev, "critical") {
		score += 0.3
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}
