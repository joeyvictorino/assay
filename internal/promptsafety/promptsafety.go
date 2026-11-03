package promptsafety

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type FlaggedPattern struct {
	Category string `json:"category"`
	Pattern  string `json:"pattern"`
	Match    string `json:"match"`
	Severity string `json:"severity"`
}

type SafetyReport struct {
	Prompt         string           `json:"prompt"`
	RiskScore      float64          `json:"risk_score"`
	Verdict        string           `json:"verdict"`
	FlaggedCount   int              `json:"flagged_count"`
	Flagged        []FlaggedPattern `json:"flagged"`
}

type BatchReport struct {
	Results   []SafetyReport `json:"results"`
	PassCount int            `json:"pass_count"`
	FailCount int            `json:"fail_count"`
	MeanRisk  float64        `json:"mean_risk"`
}

type category struct {
	name     string
	severity string
	patterns []*regexp.Regexp
}

var categories = []category{
	{
		name:     "unsafe_instruction",
		severity: "high",
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(ignore|disregard|bypass|override|forget)\s+(all\s+)?(previous|prior|above|safety|security|system)\s+(instruction|guideline|rule|constraint|prompt)`),
			regexp.MustCompile(`(?i)(do\s+not|don'?t)\s+(follow|obey|respect|enforce)\s+(safety|security|content|ethical)\s+(guideline|rule|policy|filter)`),
			regexp.MustCompile(`(?i)you\s+(are|must)\s+(now\s+)?(free|exempt|released|liberated)\s+(from|of)\s+(all\s+)?(restriction|constraint|rule|guideline)`),
			regexp.MustCompile(`(?i)(disable|turn\s+off|deactivate|remove)\s+(safety|content|ethical|security)\s+(filter|guard|check|restriction)`),
		},
	},
	{
		name:     "data_exfil_risk",
		severity: "high",
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(reveal|show|print|output|repeat|display)\s+(your|the)\s+(system|initial|original|hidden|secret)\s*(prompt|instruction|message|directive)`),
			regexp.MustCompile(`(?i)(what|tell\s+me)\s+(is|are|about)\s+(your|the)\s+(system|initial|training|original)\s*(prompt|instruction|data|set)`),
			regexp.MustCompile(`(?i)(extract|leak|exfiltrate|dump|expose)\s+(training|system|internal|private|confidential)\s+(data|information|knowledge|content)`),
			regexp.MustCompile(`(?i)(reproduce|recite|output)\s+(verbatim|exactly|word.for.word)\s+.*(training|copyrighted|proprietary)`),
		},
	},
	{
		name:     "injection_pattern",
		severity: "critical",
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)(ignore\s+previous|disregard\s+above|forget\s+everything|new\s+instructions?\s*:)`),
			regexp.MustCompile(`(?i)(system|assistant|user)\s*:\s*(you\s+are|ignore|forget|from\s+now)`),
			regexp.MustCompile(`(?i)(jailbreak|DAN|developer\s+mode|god\s+mode|unrestricted\s+mode)`),
			regexp.MustCompile(`(?i)\[SYSTEM\]|\[INST\]|\[\/INST\]|<<SYS>>|<\|system\|>`),
		},
	},
}

var severityWeight = map[string]float64{
	"critical": 1.0,
	"high":     0.7,
	"medium":   0.4,
	"low":      0.2,
}

func Analyze(prompt string) SafetyReport {
	r := SafetyReport{Prompt: truncate(prompt, 200)}

	for _, cat := range categories {
		for _, pat := range cat.patterns {
			if m := pat.FindString(prompt); m != "" {
				r.Flagged = append(r.Flagged, FlaggedPattern{
					Category: cat.name,
					Pattern:  pat.String()[:min(60, len(pat.String()))],
					Match:    truncate(m, 100),
					Severity: cat.severity,
				})
			}
		}
	}

	r.FlaggedCount = len(r.Flagged)

	if r.FlaggedCount == 0 {
		r.RiskScore = 0
		r.Verdict = "pass"
		return r
	}

	var totalWeight float64
	for _, f := range r.Flagged {
		totalWeight += severityWeight[f.Severity]
	}
	r.RiskScore = clamp(totalWeight/float64(max(1, r.FlaggedCount)), 0, 1)

	if r.RiskScore >= 0.5 {
		r.Verdict = "fail"
	} else {
		r.Verdict = "warn"
	}

	return r
}

func AnalyzeBatch(prompts []string) BatchReport {
	br := BatchReport{}
	var total float64
	for _, p := range prompts {
		r := Analyze(p)
		br.Results = append(br.Results, r)
		total += r.RiskScore
		if r.Verdict == "pass" {
			br.PassCount++
		} else {
			br.FailCount++
		}
	}
	if len(prompts) > 0 {
		br.MeanRisk = total / float64(len(prompts))
	}
	return br
}

func FormatText(r SafetyReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Prompt Safety Report\n")
	fmt.Fprintf(&b, "====================\n\n")
	fmt.Fprintf(&b, "Risk Score: %.2f\n", r.RiskScore)
	fmt.Fprintf(&b, "Verdict:    %s\n", r.Verdict)
	fmt.Fprintf(&b, "Flagged:    %d patterns\n\n", r.FlaggedCount)
	for _, f := range r.Flagged {
		fmt.Fprintf(&b, "  [%s] %s: %s\n", f.Severity, f.Category, f.Match)
	}
	return b.String()
}

func FormatJSON(r SafetyReport) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
