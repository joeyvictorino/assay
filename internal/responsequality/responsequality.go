package responsequality

import (
	"fmt"
	"math"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type DimensionScore struct {
	Name  string  `json:"name"`
	Score float64 `json:"score"`
	Note  string  `json:"note,omitempty"`
}

type FindingQuality struct {
	FindingID  string           `json:"finding_id"`
	Severity   string           `json:"severity"`
	Overall    float64          `json:"overall"`
	Grade      string           `json:"grade"`
	Dimensions []DimensionScore `json:"dimensions"`
	Flagged    bool             `json:"flagged"`
}

type Summary struct {
	TotalFindings int                `json:"total_findings"`
	MeanQuality   float64            `json:"mean_quality"`
	GradeDistrib  map[string]int     `json:"grade_distribution"`
	FlaggedCount  int                `json:"flagged_count"`
	ByDimension   map[string]float64 `json:"by_dimension"`
	LowestDim     string             `json:"lowest_dimension"`
}

type Report struct {
	Findings []FindingQuality `json:"findings"`
	Summary  Summary          `json:"summary"`
}

const (
	weightCompleteness  = 0.30
	weightSpecificity   = 0.25
	weightActionability = 0.25
	weightClarity       = 0.20
)

func Analyze(findings []model.Finding, threshold float64) Report {
	if len(findings) == 0 {
		return Report{
			Summary: Summary{
				GradeDistrib: map[string]int{},
				ByDimension:  map[string]float64{},
			},
		}
	}

	results := make([]FindingQuality, len(findings))
	for i, f := range findings {
		results[i] = scoreFinding(f, threshold)
	}

	return Report{
		Findings: results,
		Summary:  buildSummary(results),
	}
}

func scoreFinding(f model.Finding, threshold float64) FindingQuality {
	comp := scoreCompleteness(f)
	spec := scoreSpecificity(f)
	act := scoreActionability(f)
	clar := scoreClarity(f)

	overall := weightCompleteness*comp.Score +
		weightSpecificity*spec.Score +
		weightActionability*act.Score +
		weightClarity*clar.Score
	overall = math.Round(overall*100) / 100

	return FindingQuality{
		FindingID:  f.ID,
		Severity:   string(f.Severity),
		Overall:    overall,
		Grade:      letterGrade(overall),
		Dimensions: []DimensionScore{comp, spec, act, clar},
		Flagged:    overall < threshold,
	}
}

func scoreCompleteness(f model.Finding) DimensionScore {
	score := 0.0
	total := 5.0

	if f.Summary != "" {
		score++
	}
	if f.CWE != "" {
		score++
	}
	if f.Location.Method != "" || f.Location.PathTemplate != "" {
		score++
	}
	if len(f.Evidence) > 0 {
		score++
	}
	if f.Severity != "" {
		score++
	}

	s := math.Round(score/total*100) / 100
	note := ""
	if s < 0.6 {
		note = "missing key fields"
	}
	return DimensionScore{Name: "completeness", Score: s, Note: note}
}

func scoreSpecificity(f model.Finding) DimensionScore {
	s := f.Summary
	if s == "" {
		return DimensionScore{Name: "specificity", Score: 0, Note: "no summary"}
	}

	score := 0.0
	words := strings.Fields(s)

	if len(words) >= 5 {
		score += 0.3
	}
	if f.Location.Method != "" && f.Location.PathTemplate != "" {
		score += 0.3
	}
	if f.CWE != "" {
		score += 0.2
	}
	hasNumeric := false
	for _, w := range words {
		for _, c := range w {
			if c >= '0' && c <= '9' {
				hasNumeric = true
				break
			}
		}
	}
	if hasNumeric {
		score += 0.2
	}

	score = math.Min(score, 1.0)
	score = math.Round(score*100) / 100
	note := ""
	if score < 0.5 {
		note = "vague description"
	}
	return DimensionScore{Name: "specificity", Score: score, Note: note}
}

func scoreActionability(f model.Finding) DimensionScore {
	score := 0.0

	if f.Remediation != nil {
		if f.Remediation.ConfigChange != "" {
			score += 0.4
		}
		if f.Remediation.VirtualPatch != "" {
			score += 0.2
		}
	}

	if f.Location.Method != "" {
		score += 0.2
	}
	if f.Severity != "" && f.Severity.Rank() >= 2 {
		score += 0.2
	}

	score = math.Min(score, 1.0)
	score = math.Round(score*100) / 100
	note := ""
	if score < 0.4 {
		note = "no remediation guidance"
	}
	return DimensionScore{Name: "actionability", Score: score, Note: note}
}

func scoreClarity(f model.Finding) DimensionScore {
	s := f.Summary
	if s == "" {
		return DimensionScore{Name: "clarity", Score: 0, Note: "empty summary"}
	}

	words := strings.Fields(s)
	score := 0.0

	if len(words) >= 3 && len(words) <= 50 {
		score += 0.4
	} else if len(words) > 50 {
		score += 0.2
	}

	avgWordLen := 0.0
	for _, w := range words {
		avgWordLen += float64(len(w))
	}
	if len(words) > 0 {
		avgWordLen /= float64(len(words))
	}
	if avgWordLen > 0 && avgWordLen <= 10 {
		score += 0.3
	} else if avgWordLen > 10 {
		score += 0.1
	}

	if !strings.Contains(s, "  ") {
		score += 0.15
	}
	if len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z' {
		score += 0.15
	}

	score = math.Min(score, 1.0)
	score = math.Round(score*100) / 100
	return DimensionScore{Name: "clarity", Score: score}
}

func letterGrade(score float64) string {
	switch {
	case score >= 0.9:
		return "A"
	case score >= 0.8:
		return "B"
	case score >= 0.7:
		return "C"
	case score >= 0.6:
		return "D"
	default:
		return "F"
	}
}

func buildSummary(results []FindingQuality) Summary {
	grades := map[string]int{}
	dimSums := map[string]float64{}
	dimCounts := map[string]int{}
	var totalQuality float64
	var flagged int

	for _, r := range results {
		totalQuality += r.Overall
		grades[r.Grade]++
		if r.Flagged {
			flagged++
		}
		for _, d := range r.Dimensions {
			dimSums[d.Name] += d.Score
			dimCounts[d.Name]++
		}
	}

	byDim := map[string]float64{}
	lowestDim := ""
	lowestVal := 2.0
	for name, sum := range dimSums {
		avg := math.Round(sum/float64(dimCounts[name])*100) / 100
		byDim[name] = avg
		if avg < lowestVal {
			lowestVal = avg
			lowestDim = name
		}
	}

	return Summary{
		TotalFindings: len(results),
		MeanQuality:   math.Round(totalQuality/float64(len(results))*100) / 100,
		GradeDistrib:  grades,
		FlaggedCount:  flagged,
		ByDimension:   byDim,
		LowestDim:     lowestDim,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Quality Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	s := r.Summary
	fmt.Fprintf(&b, "  Total findings:     %d\n", s.TotalFindings)
	fmt.Fprintf(&b, "  Mean quality:       %.2f\n", s.MeanQuality)
	fmt.Fprintf(&b, "  Flagged (low):      %d\n", s.FlaggedCount)
	if s.LowestDim != "" {
		fmt.Fprintf(&b, "  Weakest dimension:  %s\n", s.LowestDim)
	}
	b.WriteString("\n")

	b.WriteString("  Grade Distribution:\n")
	for _, g := range []string{"A", "B", "C", "D", "F"} {
		if count, ok := s.GradeDistrib[g]; ok && count > 0 {
			fmt.Fprintf(&b, "    %s: %d\n", g, count)
		}
	}
	b.WriteString("\n")

	b.WriteString("  Dimension Averages:\n")
	for _, key := range []string{"completeness", "specificity", "actionability", "clarity"} {
		fmt.Fprintf(&b, "    %-20s%.2f\n", key+":", s.ByDimension[key])
	}
	b.WriteString("\n")

	for _, fq := range r.Findings {
		tag := "OK"
		if fq.Flagged {
			tag = "LOW"
		}
		fmt.Fprintf(&b, "  [%s] %s  grade=%s  overall=%.2f  severity=%s\n",
			tag, fq.FindingID, fq.Grade, fq.Overall, fq.Severity)
		for _, d := range fq.Dimensions {
			note := ""
			if d.Note != "" {
				note = " (" + d.Note + ")"
			}
			fmt.Fprintf(&b, "    %-16s%.2f%s\n", d.Name+":", d.Score, note)
		}
		b.WriteString("\n")
	}

	return b.String()
}
