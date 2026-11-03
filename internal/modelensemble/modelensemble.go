package modelensemble

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Vote struct {
	Model   string        `json:"model"`
	Finding model.Finding `json:"finding"`
	Agrees  bool          `json:"agrees"`
}

type ConsensusResult struct {
	FindingID string  `json:"finding_id"`
	Summary   string  `json:"summary"`
	Votes     []Vote  `json:"votes"`
	Agreement float64 `json:"agreement"`
	Unanimous bool    `json:"unanimous"`
}

type EnsembleSummary struct {
	TotalFindings    int     `json:"total_findings"`
	UnanimousCount   int     `json:"unanimous_count"`
	MajorityCount    int     `json:"majority_count"`
	DisputedCount    int     `json:"disputed_count"`
	AverageAgreement float64 `json:"average_agreement"`
}

type EnsembleReport struct {
	Results []ConsensusResult `json:"results"`
	Summary EnsembleSummary   `json:"summary"`
}

func Analyze(findings []model.Finding) EnsembleReport {
	groups := groupByDedupKey(findings)

	var results []ConsensusResult
	for key, group := range groups {
		cr := analyzeGroup(key, group)
		results = append(results, cr)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Agreement < results[j].Agreement
	})

	summary := EnsembleSummary{TotalFindings: len(results)}
	totalAgreement := 0.0
	for _, r := range results {
		totalAgreement += r.Agreement
		if r.Unanimous {
			summary.UnanimousCount++
		} else if r.Agreement >= 0.5 {
			summary.MajorityCount++
		} else {
			summary.DisputedCount++
		}
	}
	if len(results) > 0 {
		summary.AverageAgreement = totalAgreement / float64(len(results))
	}

	return EnsembleReport{Results: results, Summary: summary}
}

func groupByDedupKey(findings []model.Finding) map[string][]model.Finding {
	groups := make(map[string][]model.Finding)
	for _, f := range findings {
		key := f.DedupKey
		if key == "" {
			key = f.ID
		}
		groups[key] = append(groups[key], f)
	}
	return groups
}

func analyzeGroup(key string, findings []model.Finding) ConsensusResult {
	models := make(map[string]bool)
	for _, f := range findings {
		models[f.Model.Model] = true
	}

	validated := 0
	for _, f := range findings {
		if f.State == "validated" {
			validated++
		}
	}

	var votes []Vote
	for _, f := range findings {
		agrees := f.State == "validated" || f.State == "theorized"
		votes = append(votes, Vote{
			Model:   f.Model.Model,
			Finding: f,
			Agrees:  agrees,
		})
	}

	agreeCount := 0
	for _, v := range votes {
		if v.Agrees {
			agreeCount++
		}
	}

	agreement := 0.0
	if len(votes) > 0 {
		agreement = float64(agreeCount) / float64(len(votes))
	}

	summary := ""
	if len(findings) > 0 {
		summary = findings[0].Summary
	}

	return ConsensusResult{
		FindingID: key,
		Summary:   summary,
		Votes:     votes,
		Agreement: agreement,
		Unanimous: agreeCount == len(votes) && len(votes) > 0,
	}
}

func FormatText(r EnsembleReport) string {
	var sb strings.Builder
	sb.WriteString("Multi-Model Ensemble Assessment\n")
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&sb, "  Findings:    %d\n", r.Summary.TotalFindings)
	fmt.Fprintf(&sb, "  Unanimous:   %d\n", r.Summary.UnanimousCount)
	fmt.Fprintf(&sb, "  Majority:    %d\n", r.Summary.MajorityCount)
	fmt.Fprintf(&sb, "  Disputed:    %d\n", r.Summary.DisputedCount)
	fmt.Fprintf(&sb, "  Avg agree:   %.1f%%\n\n", r.Summary.AverageAgreement*100)

	for _, cr := range r.Results {
		status := "DISPUTED"
		if cr.Unanimous {
			status = "UNANIMOUS"
		} else if cr.Agreement >= 0.5 {
			status = "MAJORITY"
		}
		fmt.Fprintf(&sb, "  [%s] %s (%.0f%% agreement)\n", status, cr.FindingID, cr.Agreement*100)
		if cr.Summary != "" {
			fmt.Fprintf(&sb, "    %s\n", cr.Summary)
		}
		for _, v := range cr.Votes {
			agree := "AGREE"
			if !v.Agrees {
				agree = "DISAGREE"
			}
			fmt.Fprintf(&sb, "    - %s: %s (%s)\n", v.Model, agree, v.Finding.State)
		}
	}

	return sb.String()
}

func FormatJSON(r EnsembleReport) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
