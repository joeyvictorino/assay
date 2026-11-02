package coherence

import (
	"fmt"
	"math"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type TurnScore struct {
	TurnIndex         int     `json:"turn_index"`
	FindingID         string  `json:"finding_id"`
	TopicDrift        float64 `json:"topic_drift"`
	SeverityStability float64 `json:"severity_stability"`
	ClassStability    float64 `json:"class_stability"`
	DetailProgression float64 `json:"detail_progression"`
	OverallScore      float64 `json:"overall_score"`
	Flagged           bool    `json:"flagged"`
}

type Report struct {
	Scores  []TurnScore `json:"scores"`
	Summary Summary     `json:"summary"`
}

type Summary struct {
	TotalFindings int                `json:"total_findings"`
	FlaggedCount  int                `json:"flagged_count"`
	MeanScore     float64            `json:"mean_score"`
	MaxDrift      float64            `json:"max_drift"`
	ByIndicator   map[string]float64 `json:"by_indicator"`
}

const (
	weightTopic    = 0.35
	weightSeverity = 0.25
	weightClass    = 0.20
	weightDetail   = 0.20
)

func Analyze(findings []model.Finding, threshold float64) Report {
	if len(findings) == 0 {
		return Report{
			Summary: Summary{ByIndicator: map[string]float64{}},
		}
	}

	scores := make([]TurnScore, len(findings))
	for i, f := range findings {
		var prevSummary string
		var prevSeverity string
		var prevClass string
		if i > 0 {
			prevSummary = findings[i-1].Summary
			prevSeverity = string(findings[i-1].Severity)
			prevClass = string(findings[i-1].Class)
		}

		topic := scoreTopicDrift(f.Summary, prevSummary)
		sev := scoreSeverityStability(string(f.Severity), prevSeverity)
		cls := scoreClassStability(string(f.Class), prevClass)
		detail := scoreDetailProgression(f.Summary, prevSummary)

		overall := weightTopic*topic +
			weightSeverity*sev +
			weightClass*cls +
			weightDetail*detail

		scores[i] = TurnScore{
			TurnIndex:         i,
			FindingID:         f.ID,
			TopicDrift:        math.Round(topic*100) / 100,
			SeverityStability: math.Round(sev*100) / 100,
			ClassStability:    math.Round(cls*100) / 100,
			DetailProgression: math.Round(detail*100) / 100,
			OverallScore:      math.Round(overall*100) / 100,
			Flagged:           overall >= threshold,
		}
	}

	var sum, maxDrift float64
	var flagged int
	sumTopic, sumSev, sumCls, sumDetail := 0.0, 0.0, 0.0, 0.0
	for _, s := range scores {
		sum += s.OverallScore
		if s.TopicDrift > maxDrift {
			maxDrift = s.TopicDrift
		}
		if s.Flagged {
			flagged++
		}
		sumTopic += s.TopicDrift
		sumSev += s.SeverityStability
		sumCls += s.ClassStability
		sumDetail += s.DetailProgression
	}

	n := float64(len(scores))
	return Report{
		Scores: scores,
		Summary: Summary{
			TotalFindings: len(scores),
			FlaggedCount:  flagged,
			MeanScore:     math.Round(sum/n*100) / 100,
			MaxDrift:      maxDrift,
			ByIndicator: map[string]float64{
				"topic_drift":        math.Round(sumTopic/n*100) / 100,
				"severity_stability": math.Round(sumSev/n*100) / 100,
				"class_stability":    math.Round(sumCls/n*100) / 100,
				"detail_progression": math.Round(sumDetail/n*100) / 100,
			},
		},
	}
}

func scoreTopicDrift(current, previous string) float64 {
	if previous == "" {
		return 0.0
	}
	sim := jaccardSimilarity(current, previous)
	if sim >= 0.6 {
		return 0.0
	}
	if sim >= 0.3 {
		return 0.5
	}
	return 1.0
}

func scoreSeverityStability(current, previous string) float64 {
	if previous == "" {
		return 0.0
	}
	if strings.EqualFold(current, previous) {
		return 0.0
	}
	diff := math.Abs(float64(severityRank(current) - severityRank(previous)))
	if diff <= 1 {
		return 0.3
	}
	return 1.0
}

func scoreClassStability(current, previous string) float64 {
	if previous == "" {
		return 0.0
	}
	if strings.EqualFold(current, previous) {
		return 0.0
	}
	return 1.0
}

func scoreDetailProgression(current, previous string) float64 {
	if previous == "" {
		return 0.0
	}
	curWords := len(strings.Fields(current))
	prevWords := len(strings.Fields(previous))
	if prevWords == 0 {
		return 0.0
	}
	ratio := float64(curWords) / float64(prevWords)
	if ratio >= 0.5 && ratio <= 2.0 {
		return 0.0
	}
	if ratio > 3.0 || ratio < 0.3 {
		return 1.0
	}
	return 0.5
}

func severityRank(sev string) int {
	switch strings.ToLower(sev) {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	case "info":
		return 1
	default:
		return 3
	}
}

func jaccardSimilarity(a, b string) float64 {
	setA := wordSet(strings.ToLower(a))
	setB := wordSet(strings.ToLower(b))
	if len(setA) == 0 && len(setB) == 0 {
		return 0
	}
	inter := 0
	for w := range setA {
		if setB[w] {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func wordSet(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Multi-Turn Coherence Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	s := r.Summary
	fmt.Fprintf(&b, "  Total findings:     %d\n", s.TotalFindings)
	fmt.Fprintf(&b, "  Flagged findings:   %d\n", s.FlaggedCount)
	fmt.Fprintf(&b, "  Mean score:         %.2f\n", s.MeanScore)
	fmt.Fprintf(&b, "  Max topic drift:    %.2f\n", s.MaxDrift)
	b.WriteString("\n")

	b.WriteString("  Indicator Averages:\n")
	for _, key := range []string{"topic_drift", "severity_stability", "class_stability", "detail_progression"} {
		fmt.Fprintf(&b, "    %-24s%.2f\n", key+":", s.ByIndicator[key])
	}
	b.WriteString("\n")

	for _, sc := range r.Scores {
		tag := "OK"
		if sc.Flagged {
			tag = "DRIFT"
		}
		fmt.Fprintf(&b, "  [%s] turn=%d %s (score: %.2f)\n", tag, sc.TurnIndex, sc.FindingID, sc.OverallScore)
		fmt.Fprintf(&b, "    Topic: %.2f  Severity: %.2f  Class: %.2f  Detail: %.2f\n",
			sc.TopicDrift, sc.SeverityStability, sc.ClassStability, sc.DetailProgression)
		b.WriteString("\n")
	}

	return b.String()
}
