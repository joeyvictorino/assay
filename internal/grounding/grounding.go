package grounding

import (
	"fmt"
	"math"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Score struct {
	FindingID          string  `json:"finding_id"`
	EvidenceGrounding  float64 `json:"evidence_grounding"`
	LocationGrounding  float64 `json:"location_grounding"`
	SummarySpecificity float64 `json:"summary_specificity"`
	ClassGrounding     float64 `json:"class_grounding"`
	CWEGrounding       float64 `json:"cwe_grounding"`
	OverallScore       float64 `json:"overall_score"`
	Flagged            bool    `json:"flagged"`
}

type Report struct {
	Scores  []Score `json:"scores"`
	Summary Summary `json:"summary"`
}

type Summary struct {
	TotalFindings int                `json:"total_findings"`
	FlaggedCount  int                `json:"flagged_count"`
	MeanScore     float64            `json:"mean_score"`
	MinScore      float64            `json:"min_score"`
	ByIndicator   map[string]float64 `json:"by_indicator"`
}

const (
	weightEvidence = 0.30
	weightLocation = 0.25
	weightSummary  = 0.20
	weightClass    = 0.15
	weightCWE      = 0.10
)

func Analyze(findings []model.Finding, threshold float64) Report {
	if len(findings) == 0 {
		return Report{
			Summary: Summary{ByIndicator: map[string]float64{}, MinScore: 1.0},
		}
	}

	scores := make([]Score, len(findings))
	for i, f := range findings {
		ev := scoreEvidenceGrounding(f)
		loc := scoreLocationGrounding(f)
		summ := scoreSummarySpecificity(f)
		cls := scoreClassGrounding(f)
		cwe := scoreCWEGrounding(f)

		overall := weightEvidence*ev +
			weightLocation*loc +
			weightSummary*summ +
			weightClass*cls +
			weightCWE*cwe

		scores[i] = Score{
			FindingID:          f.ID,
			EvidenceGrounding:  math.Round(ev*100) / 100,
			LocationGrounding:  math.Round(loc*100) / 100,
			SummarySpecificity: math.Round(summ*100) / 100,
			ClassGrounding:     math.Round(cls*100) / 100,
			CWEGrounding:       math.Round(cwe*100) / 100,
			OverallScore:       math.Round(overall*100) / 100,
			Flagged:            overall < threshold,
		}
	}

	var sum float64
	minS := 1.0
	var flagged int
	sumEv, sumLoc, sumSumm, sumCls, sumCWE := 0.0, 0.0, 0.0, 0.0, 0.0
	for _, s := range scores {
		sum += s.OverallScore
		if s.OverallScore < minS {
			minS = s.OverallScore
		}
		if s.Flagged {
			flagged++
		}
		sumEv += s.EvidenceGrounding
		sumLoc += s.LocationGrounding
		sumSumm += s.SummarySpecificity
		sumCls += s.ClassGrounding
		sumCWE += s.CWEGrounding
	}

	n := float64(len(scores))
	return Report{
		Scores: scores,
		Summary: Summary{
			TotalFindings: len(scores),
			FlaggedCount:  flagged,
			MeanScore:     math.Round(sum/n*100) / 100,
			MinScore:      minS,
			ByIndicator: map[string]float64{
				"evidence_grounding":  math.Round(sumEv/n*100) / 100,
				"location_grounding":  math.Round(sumLoc/n*100) / 100,
				"summary_specificity": math.Round(sumSumm/n*100) / 100,
				"class_grounding":     math.Round(sumCls/n*100) / 100,
				"cwe_grounding":       math.Round(sumCWE/n*100) / 100,
			},
		},
	}
}

func scoreEvidenceGrounding(f model.Finding) float64 {
	if len(f.Evidence) == 0 {
		return 0.0
	}
	concrete := 0
	for _, e := range f.Evidence {
		if e.Marker != "" {
			concrete++
		}
		if e.StatusCode != 0 {
			concrete++
		}
		if e.RequestDigest != "" {
			concrete++
		}
	}
	if concrete >= 3 {
		return 1.0
	}
	if concrete >= 1 {
		return 0.6
	}
	return 0.3
}

func scoreLocationGrounding(f model.Finding) float64 {
	has := 0
	if f.Location.Method != "" {
		has++
	}
	if f.Location.PathTemplate != "" {
		has++
	}
	if f.Location.Param != "" {
		has++
	}
	if f.Location.Component != "" {
		has++
	}
	switch {
	case has >= 3:
		return 1.0
	case has == 2:
		return 0.7
	case has == 1:
		return 0.4
	default:
		return 0.0
	}
}

func scoreSummarySpecificity(f model.Finding) float64 {
	words := strings.Fields(f.Summary)
	if len(words) == 0 {
		return 0.0
	}
	specifics := 0
	for _, w := range words {
		lower := strings.ToLower(w)
		if isSpecificWord(lower) {
			specifics++
		}
	}
	ratio := float64(specifics) / float64(len(words))
	if ratio >= 0.3 {
		return 1.0
	}
	if ratio >= 0.15 {
		return 0.6
	}
	if ratio >= 0.05 {
		return 0.3
	}
	return 0.0
}

func isSpecificWord(w string) bool {
	if strings.HasPrefix(w, "/") || strings.HasPrefix(w, "http") {
		return true
	}
	if strings.Contains(w, "=") || strings.Contains(w, "(") {
		return true
	}
	if strings.HasPrefix(w, "cve-") || strings.HasPrefix(w, "cwe-") {
		return true
	}
	if len(w) > 3 && strings.ContainsAny(w, "0123456789") {
		return true
	}
	technicalWords := map[string]bool{
		"sql": true, "xss": true, "injection": true, "csrf": true, "ssrf": true,
		"idor": true, "rce": true, "lfi": true, "rfi": true, "xxe": true,
		"buffer": true, "overflow": true, "traversal": true, "deserialization": true,
	}
	return technicalWords[w]
}

func scoreClassGrounding(f model.Finding) float64 {
	if f.Class == "" {
		return 0.0
	}
	classLower := strings.ToLower(string(f.Class))
	summaryLower := strings.ToLower(f.Summary)

	parts := strings.FieldsFunc(classLower, func(r rune) bool { return r == '-' || r == '_' })
	matches := 0
	for _, p := range parts {
		if len(p) > 2 && strings.Contains(summaryLower, p) {
			matches++
		}
	}
	if matches >= 2 {
		return 1.0
	}
	if matches == 1 {
		return 0.5
	}
	return 0.0
}

func scoreCWEGrounding(f model.Finding) float64 {
	if f.CWE == "" {
		return 0.0
	}
	if !strings.HasPrefix(strings.ToUpper(f.CWE), "CWE-") {
		return 0.2
	}
	parts := strings.SplitN(f.CWE, "-", 2)
	if len(parts) < 2 || len(parts[1]) == 0 {
		return 0.3
	}
	for _, c := range parts[1] {
		if c < '0' || c > '9' {
			return 0.3
		}
	}
	return 1.0
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Factual Grounding Verification\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	s := r.Summary
	fmt.Fprintf(&b, "  Total findings:     %d\n", s.TotalFindings)
	fmt.Fprintf(&b, "  Poorly grounded:    %d\n", s.FlaggedCount)
	fmt.Fprintf(&b, "  Mean score:         %.2f\n", s.MeanScore)
	fmt.Fprintf(&b, "  Min score:          %.2f\n", s.MinScore)
	b.WriteString("\n")

	b.WriteString("  Indicator Averages:\n")
	for _, key := range []string{"evidence_grounding", "location_grounding", "summary_specificity", "class_grounding", "cwe_grounding"} {
		fmt.Fprintf(&b, "    %-24s%.2f\n", key+":", s.ByIndicator[key])
	}
	b.WriteString("\n")

	for _, sc := range r.Scores {
		tag := "GROUNDED"
		if sc.Flagged {
			tag = "WEAK"
		}
		fmt.Fprintf(&b, "  [%s] %s (score: %.2f)\n", tag, sc.FindingID, sc.OverallScore)
		fmt.Fprintf(&b, "    Evidence: %.2f  Location: %.2f  Summary: %.2f\n",
			sc.EvidenceGrounding, sc.LocationGrounding, sc.SummarySpecificity)
		fmt.Fprintf(&b, "    Class: %.2f  CWE: %.2f\n",
			sc.ClassGrounding, sc.CWEGrounding)
		b.WriteString("\n")
	}

	return b.String()
}
