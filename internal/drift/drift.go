package drift

import (
	"fmt"
	"math"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type RunPair struct {
	RunA string `json:"run_a"`
	RunB string `json:"run_b"`
}

type FindingDrift struct {
	FindingID      string  `json:"finding_id"`
	DedupKey       string  `json:"dedup_key"`
	SeverityDrift  float64 `json:"severity_drift"`
	SummaryDrift   float64 `json:"summary_drift"`
	EvidenceDrift  float64 `json:"evidence_drift"`
	OverallDrift   float64 `json:"overall_drift"`
	Flagged        bool    `json:"flagged"`
	StatusA        string  `json:"status_a"`
	StatusB        string  `json:"status_b"`
}

type Report struct {
	Runs     RunPair        `json:"runs"`
	Drifts   []FindingDrift `json:"drifts"`
	NewInB   []string       `json:"new_in_b"`
	GoneInB  []string       `json:"gone_in_b"`
	Summary  Summary        `json:"summary"`
}

type Summary struct {
	TotalMatched   int     `json:"total_matched"`
	TotalNew       int     `json:"total_new"`
	TotalGone      int     `json:"total_gone"`
	FlaggedCount   int     `json:"flagged_count"`
	MeanDrift      float64 `json:"mean_drift"`
	MaxDrift       float64 `json:"max_drift"`
	StabilityScore float64 `json:"stability_score"`
}

const (
	weightSeverity = 0.30
	weightSummary  = 0.40
	weightEvidence = 0.30
)

func Analyze(runA, runB []model.Finding, runAName, runBName string, threshold float64) Report {
	mapA := buildDedupMap(runA)
	mapB := buildDedupMap(runB)

	var drifts []FindingDrift
	var newInB, goneInB []string

	for key, fb := range mapB {
		fa, ok := mapA[key]
		if !ok {
			newInB = append(newInB, key)
			continue
		}
		d := compareFinding(fa, fb, threshold)
		drifts = append(drifts, d)
	}

	for key := range mapA {
		if _, ok := mapB[key]; !ok {
			goneInB = append(goneInB, key)
		}
	}

	summary := buildSummary(drifts, newInB, goneInB)

	return Report{
		Runs:    RunPair{RunA: runAName, RunB: runBName},
		Drifts:  drifts,
		NewInB:  newInB,
		GoneInB: goneInB,
		Summary: summary,
	}
}

func buildDedupMap(findings []model.Finding) map[string]model.Finding {
	m := make(map[string]model.Finding)
	for _, f := range findings {
		key := f.DedupKey
		if key == "" {
			key = f.ID
		}
		m[key] = f
	}
	return m
}

func compareFinding(a, b model.Finding, threshold float64) FindingDrift {
	sevDrift := severityDrift(string(a.Severity), string(b.Severity))
	sumDrift := 1.0 - jaccardSimilarity(a.Summary, b.Summary)
	evDrift := evidenceDrift(a, b)

	overall := weightSeverity*sevDrift + weightSummary*sumDrift + weightEvidence*evDrift

	key := a.DedupKey
	if key == "" {
		key = a.ID
	}

	return FindingDrift{
		FindingID:     key,
		DedupKey:      key,
		SeverityDrift: round2(sevDrift),
		SummaryDrift:  round2(sumDrift),
		EvidenceDrift: round2(evDrift),
		OverallDrift:  round2(overall),
		Flagged:       overall >= threshold,
		StatusA:       string(a.State),
		StatusB:       string(b.State),
	}
}

var severityRank = map[string]int{
	"info": 0, "low": 1, "medium": 2, "high": 3, "critical": 4,
}

func severityDrift(a, b string) float64 {
	ra := severityRank[strings.ToLower(a)]
	rb := severityRank[strings.ToLower(b)]
	diff := math.Abs(float64(ra - rb))
	return diff / 4.0
}

func evidenceToString(evs []model.Evidence) string {
	parts := make([]string, len(evs))
	for i, e := range evs {
		parts[i] = fmt.Sprintf("%s %s %d %s", e.Kind, e.ToolCallID, e.StatusCode, e.Marker)
	}
	return strings.Join(parts, " ")
}

func evidenceDrift(a, b model.Finding) float64 {
	evA := evidenceToString(a.Evidence)
	evB := evidenceToString(b.Evidence)
	if evA == "" && evB == "" {
		return 0
	}
	return 1.0 - jaccardSimilarity(evA, evB)
}

func jaccardSimilarity(a, b string) float64 {
	if a == "" && b == "" {
		return 0
	}
	wordsA := tokenize(a)
	wordsB := tokenize(b)
	if len(wordsA) == 0 && len(wordsB) == 0 {
		return 0
	}

	inter := 0
	for w := range wordsA {
		if wordsB[w] {
			inter++
		}
	}
	union := len(wordsA) + len(wordsB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func tokenize(s string) map[string]bool {
	words := strings.Fields(strings.ToLower(s))
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

func buildSummary(drifts []FindingDrift, newInB, goneInB []string) Summary {
	s := Summary{
		TotalMatched: len(drifts),
		TotalNew:     len(newInB),
		TotalGone:    len(goneInB),
	}

	if len(drifts) == 0 {
		s.StabilityScore = 1.0
		return s
	}

	var total float64
	for _, d := range drifts {
		total += d.OverallDrift
		if d.Flagged {
			s.FlaggedCount++
		}
		if d.OverallDrift > s.MaxDrift {
			s.MaxDrift = d.OverallDrift
		}
	}
	s.MeanDrift = round2(total / float64(len(drifts)))
	s.MaxDrift = round2(s.MaxDrift)
	s.StabilityScore = round2(1.0 - s.MeanDrift)

	return s
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Model Output Drift Report\n")
	b.WriteString(strings.Repeat("=", 40) + "\n")
	fmt.Fprintf(&b, "Run A: %s\n", r.Runs.RunA)
	fmt.Fprintf(&b, "Run B: %s\n", r.Runs.RunB)
	fmt.Fprintf(&b, "Matched findings:   %d\n", r.Summary.TotalMatched)
	fmt.Fprintf(&b, "New in B:           %d\n", r.Summary.TotalNew)
	fmt.Fprintf(&b, "Gone in B:          %d\n", r.Summary.TotalGone)
	fmt.Fprintf(&b, "Flagged:            %d\n", r.Summary.FlaggedCount)
	fmt.Fprintf(&b, "Mean drift:         %.2f\n", r.Summary.MeanDrift)
	fmt.Fprintf(&b, "Max drift:          %.2f\n", r.Summary.MaxDrift)
	fmt.Fprintf(&b, "Stability score:    %.2f\n\n", r.Summary.StabilityScore)

	for _, d := range r.Drifts {
		if !d.Flagged {
			continue
		}
		fmt.Fprintf(&b, "  [DRIFT] %s  overall=%.2f sev=%.2f sum=%.2f ev=%.2f\n",
			d.FindingID, d.OverallDrift, d.SeverityDrift, d.SummaryDrift, d.EvidenceDrift)
	}

	if len(r.NewInB) > 0 {
		b.WriteString("\nNew findings in B:\n")
		for _, id := range r.NewInB {
			fmt.Fprintf(&b, "  + %s\n", id)
		}
	}
	if len(r.GoneInB) > 0 {
		b.WriteString("\nFindings gone in B:\n")
		for _, id := range r.GoneInB {
			fmt.Fprintf(&b, "  - %s\n", id)
		}
	}

	return b.String()
}
