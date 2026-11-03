package hallucination

import (
	"fmt"
	"math"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Score struct {
	FindingID           string  `json:"finding_id"`
	EvidenceQuality     float64 `json:"evidence_quality"`
	LocationSpecificity float64 `json:"location_specificity"`
	SeverityCalibration float64 `json:"severity_calibration"`
	ClassConsistency    float64 `json:"class_consistency"`
	DuplicatePattern    float64 `json:"duplicate_pattern"`
	OverallScore        float64 `json:"overall_score"`
	Flagged             bool    `json:"flagged"`
}

type Report struct {
	Scores  []Score `json:"scores"`
	Summary Summary `json:"summary"`
}

type Summary struct {
	TotalFindings int                `json:"total_findings"`
	FlaggedCount  int                `json:"flagged_count"`
	MeanScore     float64            `json:"mean_score"`
	MaxScore      float64            `json:"max_score"`
	ByIndicator   map[string]float64 `json:"by_indicator"`
}

const (
	weightEvidence  = 0.30
	weightLocation  = 0.25
	weightSeverity  = 0.20
	weightClass     = 0.15
	weightDuplicate = 0.10
)

func Analyze(findings []model.Finding, threshold float64) Report {
	if len(findings) == 0 {
		return Report{
			Summary: Summary{ByIndicator: map[string]float64{}},
		}
	}

	summaries := make([]string, len(findings))
	for i, f := range findings {
		summaries[i] = f.Summary
	}

	scores := make([]Score, len(findings))
	for i, f := range findings {
		ev := scoreEvidence(f)
		loc := scoreLocation(f)
		sev := scoreSeverity(f)
		cls := scoreClass(f)
		dup := scoreDuplicate(f.Summary, summaries, i)

		overall := weightEvidence*ev +
			weightLocation*loc +
			weightSeverity*sev +
			weightClass*cls +
			weightDuplicate*dup

		scores[i] = Score{
			FindingID:           f.ID,
			EvidenceQuality:     ev,
			LocationSpecificity: loc,
			SeverityCalibration: sev,
			ClassConsistency:    cls,
			DuplicatePattern:    dup,
			OverallScore:        math.Round(overall*100) / 100,
			Flagged:             overall >= threshold,
		}
	}

	var sum, maxS float64
	var flagged int
	sumEv, sumLoc, sumSev, sumCls, sumDup := 0.0, 0.0, 0.0, 0.0, 0.0
	for _, s := range scores {
		sum += s.OverallScore
		if s.OverallScore > maxS {
			maxS = s.OverallScore
		}
		if s.Flagged {
			flagged++
		}
		sumEv += s.EvidenceQuality
		sumLoc += s.LocationSpecificity
		sumSev += s.SeverityCalibration
		sumCls += s.ClassConsistency
		sumDup += s.DuplicatePattern
	}

	n := float64(len(scores))
	return Report{
		Scores: scores,
		Summary: Summary{
			TotalFindings: len(scores),
			FlaggedCount:  flagged,
			MeanScore:     math.Round(sum/n*100) / 100,
			MaxScore:      maxS,
			ByIndicator: map[string]float64{
				"evidence_quality":     math.Round(sumEv/n*100) / 100,
				"location_specificity": math.Round(sumLoc/n*100) / 100,
				"severity_calibration": math.Round(sumSev/n*100) / 100,
				"class_consistency":    math.Round(sumCls/n*100) / 100,
				"duplicate_pattern":    math.Round(sumDup/n*100) / 100,
			},
		},
	}
}

func scoreEvidence(f model.Finding) float64 {
	if len(f.Evidence) == 0 {
		return 1.0
	}
	concrete := 0
	for _, e := range f.Evidence {
		if e.Marker != "" || e.StatusCode != 0 || e.RequestDigest != "" {
			concrete++
		}
	}
	if concrete >= 3 {
		return 0.0
	}
	if concrete >= 1 {
		return 0.5
	}
	return 0.8
}

func scoreLocation(f model.Finding) float64 {
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
		return 0.0
	case has == 2:
		return 0.3
	case has == 1:
		return 0.5
	default:
		return 1.0
	}
}

func scoreSeverity(f model.Finding) float64 {
	sev := string(f.Severity)
	lower := strings.ToLower(f.Summary)

	sevRank := severityRank(sev)

	contentRank := estimateContentSeverity(lower)

	diff := math.Abs(float64(sevRank - contentRank))
	if diff <= 1 {
		return 0.0
	}
	if diff == 2 {
		return 0.5
	}
	return 1.0
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

func estimateContentSeverity(lower string) int {
	critical := []string{"remote code execution", "rce", "sql injection", "authentication bypass", "privilege escalation"}
	high := []string{"cross-site scripting", "xss", "unauthorized access", "credential", "injection", "overflow"}
	low := []string{"informational", "style", "cosmetic", "minor", "deprecated", "naming"}

	for _, kw := range critical {
		if strings.Contains(lower, kw) {
			return 5
		}
	}
	for _, kw := range high {
		if strings.Contains(lower, kw) {
			return 4
		}
	}
	for _, kw := range low {
		if strings.Contains(lower, kw) {
			return 1
		}
	}
	return 3
}

func scoreClass(f model.Finding) float64 {
	if f.Class == "" {
		return 0.5
	}
	classLower := strings.ToLower(string(f.Class))
	summaryLower := strings.ToLower(f.Summary)

	keywords := classKeywords(classLower)
	if len(keywords) == 0 {
		return 0.3
	}

	matches := 0
	for _, kw := range keywords {
		if strings.Contains(summaryLower, kw) {
			matches++
		}
	}
	if matches >= 2 {
		return 0.0
	}
	if matches == 1 {
		return 0.2
	}
	return 0.8
}

var classKeywordMap = map[string][]string{
	"sqli":           {"sql", "injection", "query", "database"},
	"xss-reflected":  {"xss", "cross-site", "script", "reflected"},
	"xss-stored":     {"xss", "cross-site", "script", "stored"},
	"idor":           {"idor", "insecure direct", "object reference", "authorization"},
	"auth-missing":   {"auth", "authentication", "unauthenticated", "missing"},
	"auth-bypass":    {"auth", "bypass", "authentication"},
	"default-creds":  {"default", "credential", "password"},
	"open-redirect":  {"redirect", "open redirect", "url"},
	"ssrf":           {"ssrf", "server-side", "request forgery"},
	"path-traversal": {"path", "traversal", "directory", "../"},
	"cmd-injection":  {"command", "injection", "exec", "shell"},
}

func classKeywords(class string) []string {
	if kws, ok := classKeywordMap[class]; ok {
		return kws
	}
	parts := strings.FieldsFunc(class, func(r rune) bool { return r == '-' || r == '_' })
	return parts
}

func scoreDuplicate(summary string, allSummaries []string, selfIdx int) float64 {
	if summary == "" {
		return 0.0
	}
	var maxSim float64
	for i, other := range allSummaries {
		if i == selfIdx || other == "" {
			continue
		}
		sim := jaccardSimilarity(summary, other)
		if sim > maxSim {
			maxSim = sim
		}
	}
	if maxSim >= 0.8 {
		return 1.0
	}
	if maxSim >= 0.5 {
		return 0.5
	}
	return 0.0
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
	b.WriteString("Hallucination Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	s := r.Summary
	fmt.Fprintf(&b, "  Total findings:     %d\n", s.TotalFindings)
	fmt.Fprintf(&b, "  Flagged findings:   %d\n", s.FlaggedCount)
	fmt.Fprintf(&b, "  Mean score:         %.2f\n", s.MeanScore)
	fmt.Fprintf(&b, "  Max score:          %.2f\n", s.MaxScore)
	b.WriteString("\n")

	b.WriteString("  Indicator Averages:\n")
	for _, key := range []string{"evidence_quality", "location_specificity", "severity_calibration", "class_consistency", "duplicate_pattern"} {
		label := indicatorLabel(key)
		fmt.Fprintf(&b, "    %-24s%.2f\n", label+":", s.ByIndicator[key])
	}
	b.WriteString("\n")

	for _, sc := range r.Scores {
		tag := "OK"
		if sc.Flagged {
			tag = "FLAGGED"
		}
		fmt.Fprintf(&b, "  [%s] %s (score: %.2f)\n", tag, sc.FindingID, sc.OverallScore)
		fmt.Fprintf(&b, "    Evidence: %.2f  Location: %.2f  Severity: %.2f\n",
			sc.EvidenceQuality, sc.LocationSpecificity, sc.SeverityCalibration)
		fmt.Fprintf(&b, "    Class: %.2f  Duplicate: %.2f\n",
			sc.ClassConsistency, sc.DuplicatePattern)
		b.WriteString("\n")
	}

	return b.String()
}

func indicatorLabel(key string) string {
	switch key {
	case "evidence_quality":
		return "Evidence quality"
	case "location_specificity":
		return "Location specificity"
	case "severity_calibration":
		return "Severity calibration"
	case "class_consistency":
		return "Class consistency"
	case "duplicate_pattern":
		return "Duplicate pattern"
	default:
		return key
	}
}
