package replaydiff

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ChangedFinding struct {
	Finding  model.Finding `json:"finding"`
	OldScore float64       `json:"old_score"`
	NewScore float64       `json:"new_score"`
}

type DiffResult struct {
	BaseRunID string           `json:"base_run_id"`
	CurRunID  string           `json:"current_run_id"`
	New       []model.Finding  `json:"new"`
	Resolved  []model.Finding  `json:"resolved"`
	Changed   []ChangedFinding `json:"changed"`
	Summary   DiffSummary      `json:"summary"`
}

type DiffSummary struct {
	NewCount      int `json:"new_count"`
	ResolvedCount int `json:"resolved_count"`
	ChangedCount  int `json:"changed_count"`
}

func findingKey(f model.Finding) string {
	parts := []string{f.Lab, string(f.Model.Model), f.PromptHash}
	if f.DedupKey != "" {
		parts = append(parts, f.DedupKey)
	}
	return strings.Join(parts, "|")
}

func findingScore(f model.Finding) float64 {
	return float64(f.Severity.Rank())
}

func Diff(baseRun, curRun model.RunReport, baseFindings, curFindings []model.Finding) DiffResult {
	baseMap := make(map[string]model.Finding, len(baseFindings))
	for _, f := range baseFindings {
		baseMap[findingKey(f)] = f
	}

	curMap := make(map[string]model.Finding, len(curFindings))
	for _, f := range curFindings {
		curMap[findingKey(f)] = f
	}

	var newFindings []model.Finding
	var changed []ChangedFinding
	for key, cf := range curMap {
		bf, exists := baseMap[key]
		if !exists {
			newFindings = append(newFindings, cf)
			continue
		}
		oldScore := findingScore(bf)
		newScore := findingScore(cf)
		if oldScore != newScore {
			changed = append(changed, ChangedFinding{
				Finding:  cf,
				OldScore: oldScore,
				NewScore: newScore,
			})
		}
	}

	var resolved []model.Finding
	for key, bf := range baseMap {
		if _, exists := curMap[key]; !exists {
			resolved = append(resolved, bf)
		}
	}

	sort.Slice(newFindings, func(i, j int) bool {
		return newFindings[i].Severity.Rank() > newFindings[j].Severity.Rank()
	})
	sort.Slice(resolved, func(i, j int) bool {
		return resolved[i].Severity.Rank() > resolved[j].Severity.Rank()
	})
	sort.Slice(changed, func(i, j int) bool {
		return changed[i].Finding.Severity.Rank() > changed[j].Finding.Severity.Rank()
	})

	return DiffResult{
		BaseRunID: baseRun.RunID,
		CurRunID:  curRun.RunID,
		New:       newFindings,
		Resolved:  resolved,
		Changed:   changed,
		Summary: DiffSummary{
			NewCount:      len(newFindings),
			ResolvedCount: len(resolved),
			ChangedCount:  len(changed),
		},
	}
}

func FormatText(d DiffResult) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Replay Diff: %s → %s\n", d.BaseRunID, d.CurRunID)
	fmt.Fprintf(&b, "  New: %d  Resolved: %d  Changed: %d\n\n", d.Summary.NewCount, d.Summary.ResolvedCount, d.Summary.ChangedCount)

	if len(d.New) > 0 {
		fmt.Fprintf(&b, "New Findings (%d):\n", len(d.New))
		for _, f := range d.New {
			fmt.Fprintf(&b, "  + [%s] %s (lab=%s model=%s)\n", f.Severity, f.Summary, f.Lab, f.Model.Model)
		}
		b.WriteString("\n")
	}

	if len(d.Resolved) > 0 {
		fmt.Fprintf(&b, "Resolved Findings (%d):\n", len(d.Resolved))
		for _, f := range d.Resolved {
			fmt.Fprintf(&b, "  - [%s] %s (lab=%s model=%s)\n", f.Severity, f.Summary, f.Lab, f.Model.Model)
		}
		b.WriteString("\n")
	}

	if len(d.Changed) > 0 {
		fmt.Fprintf(&b, "Changed Findings (%d):\n", len(d.Changed))
		for _, c := range d.Changed {
			fmt.Fprintf(&b, "  ~ [%s] %s  score: %.0f → %.0f (lab=%s model=%s)\n",
				c.Finding.Severity, c.Finding.Summary, c.OldScore, c.NewScore, c.Finding.Lab, c.Finding.Model.Model)
		}
		b.WriteString("\n")
	}

	if d.Summary.NewCount == 0 && d.Summary.ResolvedCount == 0 && d.Summary.ChangedCount == 0 {
		b.WriteString("No differences found.\n")
	}

	return b.String()
}
