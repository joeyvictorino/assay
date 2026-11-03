package promptcat

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type PromptEntry struct {
	Hash       string   `json:"hash"`
	Models     []string `json:"models"`
	UsageCount int      `json:"usage_count"`
	Classes    []string `json:"classes"`
	ValidRate  float64  `json:"validation_rate"`
	AvgSev     float64  `json:"avg_severity"`
}

type DuplicateGroup struct {
	Hashes []string `json:"hashes"`
	Reason string   `json:"reason"`
}

type CatalogReport struct {
	Entries    []PromptEntry    `json:"entries"`
	Duplicates []DuplicateGroup `json:"duplicates,omitempty"`
	Total      int              `json:"total_prompts"`
	UniqueHash int              `json:"unique_hashes"`
}

type bucket struct {
	models    map[string]bool
	classes   map[string]bool
	total     int
	validated int
	sevSum    float64
}

func Analyze(findings []model.Finding) *CatalogReport {
	buckets := make(map[string]*bucket)
	for _, f := range findings {
		h := f.PromptHash
		if h == "" {
			h = "(no-hash)"
		}
		b, ok := buckets[h]
		if !ok {
			b = &bucket{
				models:  make(map[string]bool),
				classes: make(map[string]bool),
			}
			buckets[h] = b
		}
		b.models[f.Model.Model] = true
		b.classes[string(f.Class)] = true
		b.total++
		if f.State == model.StateValidated {
			b.validated++
		}
		b.sevSum += severityNum(f.Severity)
	}

	var entries []PromptEntry
	for h, b := range buckets {
		validRate := 0.0
		if b.total > 0 {
			validRate = round2(float64(b.validated) / float64(b.total) * 100)
		}
		avgSev := 0.0
		if b.total > 0 {
			avgSev = round2(b.sevSum / float64(b.total))
		}
		entries = append(entries, PromptEntry{
			Hash:       h,
			Models:     sortedKeys(b.models),
			UsageCount: b.total,
			Classes:    sortedKeys(b.classes),
			ValidRate:  validRate,
			AvgSev:     avgSev,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].UsageCount != entries[j].UsageCount {
			return entries[i].UsageCount > entries[j].UsageCount
		}
		return entries[i].Hash < entries[j].Hash
	})

	dups := findDuplicates(entries)

	return &CatalogReport{
		Entries:    entries,
		Duplicates: dups,
		Total:      len(findings),
		UniqueHash: len(entries),
	}
}

func findDuplicates(entries []PromptEntry) []DuplicateGroup {
	type sig struct {
		classes string
		models  string
	}
	groups := make(map[sig][]string)
	for _, e := range entries {
		s := sig{
			classes: strings.Join(e.Classes, ","),
			models:  strings.Join(e.Models, ","),
		}
		groups[s] = append(groups[s], e.Hash)
	}

	var dups []DuplicateGroup
	for s, hashes := range groups {
		if len(hashes) < 2 {
			continue
		}
		sort.Strings(hashes)
		dups = append(dups, DuplicateGroup{
			Hashes: hashes,
			Reason: fmt.Sprintf("same classes (%s) and models (%s)", s.classes, s.models),
		})
	}
	sort.Slice(dups, func(i, j int) bool {
		return len(dups[i].Hashes) > len(dups[j].Hashes)
	})
	return dups
}

func severityNum(s model.Severity) float64 {
	switch s {
	case model.SevCritical:
		return 4.0
	case model.SevHigh:
		return 3.0
	case model.SevMedium:
		return 2.0
	case model.SevLow:
		return 1.0
	default:
		return 0.0
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func FormatText(cr *CatalogReport) string {
	var b strings.Builder
	b.WriteString("Prompt Catalog\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	if len(cr.Entries) == 0 {
		b.WriteString("  No prompts found.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Total findings:    %d\n", cr.Total)
	fmt.Fprintf(&b, "  Unique prompts:    %d\n\n", cr.UniqueHash)

	for i, e := range cr.Entries {
		fmt.Fprintf(&b, "  [%d] hash=%s\n", i+1, ellipsis(e.Hash, 40))
		fmt.Fprintf(&b, "      Models: %s\n", strings.Join(e.Models, ", "))
		fmt.Fprintf(&b, "      Classes: %s\n", strings.Join(e.Classes, ", "))
		fmt.Fprintf(&b, "      Usage: %d  Valid: %.0f%%  AvgSev: %.1f\n\n",
			e.UsageCount, e.ValidRate, e.AvgSev)
	}

	if len(cr.Duplicates) > 0 {
		b.WriteString("  Potential Duplicates\n")
		b.WriteString("  " + strings.Repeat("-", 40) + "\n")
		for _, d := range cr.Duplicates {
			fmt.Fprintf(&b, "    %s: %s\n", strings.Join(d.Hashes, ", "), d.Reason)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(cr *CatalogReport) string {
	data, _ := json.MarshalIndent(cr, "", "  ")
	return string(data) + "\n"
}

func ellipsis(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
