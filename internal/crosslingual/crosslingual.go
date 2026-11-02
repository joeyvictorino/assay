package crosslingual

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type FindingMatch struct {
	DedupKey   string          `json:"dedup_key"`
	Languages  []string        `json:"languages"`
	Coverage   float64         `json:"coverage"`
	Severities map[string]string `json:"severities"`
	Consistent bool            `json:"consistent"`
}

type LanguageStats struct {
	Language string `json:"language"`
	Count    int    `json:"count"`
	Unique   int    `json:"unique"`
}

type Report struct {
	Languages       []LanguageStats  `json:"languages"`
	Matches         []FindingMatch   `json:"matches"`
	UniversalCount  int              `json:"universal_count"`
	PartialCount    int              `json:"partial_count"`
	UniqueCount     int              `json:"unique_count"`
	ConsistencyPct  float64          `json:"consistency_pct"`
	SeverityAgree   float64          `json:"severity_agreement_pct"`
	TotalFindings   int              `json:"total_findings"`
	TotalLanguages  int              `json:"total_languages"`
}

func Analyze(byLang map[string][]model.Finding) Report {
	if len(byLang) == 0 {
		return Report{}
	}

	langs := sortedKeys(byLang)
	totalLangs := len(langs)

	dedupByLang := make(map[string]map[string]model.Finding)
	for lang, findings := range byLang {
		dedupByLang[lang] = indexByDedup(findings)
	}

	allKeys := make(map[string]bool)
	for _, m := range dedupByLang {
		for k := range m {
			allKeys[k] = true
		}
	}

	var matches []FindingMatch
	var universalCount, partialCount, uniqueCount int
	sevAgreeCount, sevTotalCount := 0, 0

	for _, key := range sortedBoolKeys(allKeys) {
		var present []string
		sevs := make(map[string]string)
		for _, lang := range langs {
			if f, ok := dedupByLang[lang][key]; ok {
				present = append(present, lang)
				sevs[lang] = string(f.Severity)
			}
		}

		coverage := float64(len(present)) / float64(totalLangs) * 100

		consistent := true
		if len(present) > 1 {
			first := sevs[present[0]]
			for _, lang := range present[1:] {
				if sevs[lang] != first {
					consistent = false
					break
				}
			}
			for i := 1; i < len(present); i++ {
				sevTotalCount++
				if sevs[present[i]] == sevs[present[0]] {
					sevAgreeCount++
				}
			}
		}

		matches = append(matches, FindingMatch{
			DedupKey:   key,
			Languages:  present,
			Coverage:   coverage,
			Severities: sevs,
			Consistent: consistent,
		})

		switch {
		case len(present) == totalLangs:
			universalCount++
		case len(present) == 1:
			uniqueCount++
		default:
			partialCount++
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Coverage > matches[j].Coverage
	})

	var langStats []LanguageStats
	totalFindings := 0
	for _, lang := range langs {
		count := len(byLang[lang])
		totalFindings += count
		unique := 0
		for _, m := range matches {
			if len(m.Languages) == 1 && m.Languages[0] == lang {
				unique++
			}
		}
		langStats = append(langStats, LanguageStats{
			Language: lang,
			Count:    count,
			Unique:   unique,
		})
	}

	var consistencyPct float64
	if len(allKeys) > 0 {
		consistencyPct = float64(universalCount) / float64(len(allKeys)) * 100
	}

	var sevAgreePct float64
	if sevTotalCount > 0 {
		sevAgreePct = float64(sevAgreeCount) / float64(sevTotalCount) * 100
	}

	return Report{
		Languages:      langStats,
		Matches:        matches,
		UniversalCount: universalCount,
		PartialCount:   partialCount,
		UniqueCount:    uniqueCount,
		ConsistencyPct: consistencyPct,
		SeverityAgree:  sevAgreePct,
		TotalFindings:  totalFindings,
		TotalLanguages: totalLangs,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Cross-Lingual Consistency Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")
	fmt.Fprintf(&b, "  Languages analyzed: %d\n", r.TotalLanguages)
	fmt.Fprintf(&b, "  Total findings:     %d\n\n", r.TotalFindings)

	if len(r.Languages) == 0 {
		b.WriteString("  No cross-lingual data provided.\n")
		return b.String()
	}

	b.WriteString("  Per-language breakdown:\n")
	for _, ls := range r.Languages {
		fmt.Fprintf(&b, "    %s: %d findings (%d unique)\n", ls.Language, ls.Count, ls.Unique)
	}
	b.WriteString("\n")

	fmt.Fprintf(&b, "  Universal findings (all langs): %d\n", r.UniversalCount)
	fmt.Fprintf(&b, "  Partial findings (some langs):  %d\n", r.PartialCount)
	fmt.Fprintf(&b, "  Unique findings (one lang):     %d\n\n", r.UniqueCount)

	fmt.Fprintf(&b, "  Cross-lingual consistency: %.1f%%\n", r.ConsistencyPct)
	fmt.Fprintf(&b, "  Severity agreement:        %.1f%%\n", r.SeverityAgree)

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}

func indexByDedup(findings []model.Finding) map[string]model.Finding {
	m := make(map[string]model.Finding, len(findings))
	for _, f := range findings {
		key := f.DedupKey
		if key == "" {
			key = f.ID
		}
		m[key] = f
	}
	return m
}

func sortedKeys(m map[string][]model.Finding) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedBoolKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
