package normalize

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type NormalizedFinding struct {
	CanonicalID string         `json:"canonical_id"`
	DedupKey    string         `json:"dedup_key"`
	Rule        string         `json:"rule"`
	Severity    model.Severity `json:"severity"`
	Class       model.Class    `json:"class"`
	Location    model.Location `json:"location"`
	Summary     string         `json:"summary"`
	Model       model.ModelRef `json:"model"`
	Lab         string         `json:"lab"`
	OriginalIDs []string       `json:"original_ids,omitempty"`
	MergeCount  int            `json:"merge_count"`
}

type Report struct {
	TotalInput      int                 `json:"total_input"`
	TotalOutput     int                 `json:"total_output"`
	DuplicatesFound int                 `json:"duplicates_found"`
	SeverityCounts  map[string]int      `json:"severity_counts"`
	Findings        []NormalizedFinding `json:"findings"`
}

var severityAliases = map[string]model.Severity{
	"critical": model.SevCritical,
	"crit":     model.SevCritical,
	"high":     model.SevHigh,
	"h":        model.SevHigh,
	"medium":   model.SevMedium,
	"med":      model.SevMedium,
	"m":        model.SevMedium,
	"low":      model.SevLow,
	"l":        model.SevLow,
	"info":     model.SevInfo,
	"informational": model.SevInfo,
	"note":     model.SevInfo,
	"none":     model.SevInfo,
}

func normalizeSeverity(s model.Severity) model.Severity {
	if norm, ok := severityAliases[strings.ToLower(string(s))]; ok {
		return norm
	}
	return model.SevInfo
}

func dedupKey(f model.Finding) string {
	return fmt.Sprintf("%s|%s|%s|%s",
		string(f.Class),
		f.Location.Method,
		f.Location.PathTemplate,
		f.Location.Param,
	)
}

func canonicalID(f model.Finding, idx int) string {
	return fmt.Sprintf("ASSAY-%s-%04d", strings.ToUpper(string(f.Class)), idx)
}

func Analyze(findings []model.Finding) Report {
	type group struct {
		key     string
		members []model.Finding
	}

	groups := map[string]*group{}
	var order []string
	for _, f := range findings {
		k := dedupKey(f)
		if g, ok := groups[k]; ok {
			g.members = append(g.members, f)
		} else {
			groups[k] = &group{key: k, members: []model.Finding{f}}
			order = append(order, k)
		}
	}

	var normalized []NormalizedFinding
	sevCounts := map[string]int{}

	for i, k := range order {
		g := groups[k]
		representative := g.members[0]
		sev := normalizeSeverity(representative.Severity)
		var origIDs []string
		for _, m := range g.members {
			if m.ID != "" {
				origIDs = append(origIDs, m.ID)
			}
		}

		nf := NormalizedFinding{
			CanonicalID: canonicalID(representative, i+1),
			DedupKey:    k,
			Rule:        string(representative.Class),
			Severity:    sev,
			Class:       representative.Class,
			Location:    representative.Location,
			Summary:     representative.Summary,
			Model:       representative.Model,
			Lab:         representative.Lab,
			OriginalIDs: origIDs,
			MergeCount:  len(g.members),
		}
		normalized = append(normalized, nf)
		sevCounts[string(sev)]++
	}

	sort.Slice(normalized, func(i, j int) bool {
		ri := normalized[i].Severity.Rank()
		rj := normalized[j].Severity.Rank()
		if ri != rj {
			return ri > rj
		}
		return normalized[i].CanonicalID < normalized[j].CanonicalID
	})

	return Report{
		TotalInput:      len(findings),
		TotalOutput:     len(normalized),
		DuplicatesFound: len(findings) - len(normalized),
		SeverityCounts:  sevCounts,
		Findings:        normalized,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Findings Normalization Report\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&b, "  Input findings:    %d\n", r.TotalInput)
	fmt.Fprintf(&b, "  Output findings:   %d\n", r.TotalOutput)
	fmt.Fprintf(&b, "  Duplicates merged: %d\n\n", r.DuplicatesFound)

	fmt.Fprintf(&b, "  Severity breakdown:\n")
	for _, sev := range []string{"critical", "high", "medium", "low", "info"} {
		if c, ok := r.SeverityCounts[sev]; ok {
			fmt.Fprintf(&b, "    %-10s %d\n", sev, c)
		}
	}
	b.WriteString("\n")

	for _, f := range r.Findings {
		fmt.Fprintf(&b, "  %s [%s] %s\n", f.CanonicalID, f.Severity, f.Summary)
		if f.MergeCount > 1 {
			fmt.Fprintf(&b, "    merged %d duplicates\n", f.MergeCount)
		}
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
