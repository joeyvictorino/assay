package modelcompare

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type FindingKey struct {
	DedupKey string
	Lab      string
}

type FindingPair struct {
	Key    FindingKey      `json:"key"`
	RunA   *model.Finding  `json:"run_a,omitempty"`
	RunB   *model.Finding  `json:"run_b,omitempty"`
}

type SeverityAlignment struct {
	Matched    int `json:"matched"`
	Mismatched int `json:"mismatched"`
	Total      int `json:"total"`
}

type CompareResult struct {
	RunA            string            `json:"run_a"`
	RunB            string            `json:"run_b"`
	ModelA          string            `json:"model_a"`
	ModelB          string            `json:"model_b"`
	TotalA          int               `json:"total_a"`
	TotalB          int               `json:"total_b"`
	Agreed          []FindingPair     `json:"agreed"`
	OnlyA           []FindingPair     `json:"only_a"`
	OnlyB           []FindingPair     `json:"only_b"`
	AgreementRate   float64           `json:"agreement_rate"`
	DisagreementRate float64          `json:"disagreement_rate"`
	SeverityAlign   SeverityAlignment `json:"severity_alignment"`
}

func keyFor(f model.Finding) FindingKey {
	k := f.DedupKey
	if k == "" {
		k = f.ID
	}
	return FindingKey{DedupKey: k, Lab: f.Lab}
}

func primaryModel(rr model.RunReport) string {
	if len(rr.Models) == 0 {
		return "unknown"
	}
	m := rr.Models[0].Ref
	if m.Provider != "" {
		return m.Provider + "/" + m.Model
	}
	return m.Model
}

type RunData struct {
	Report   model.RunReport
	Findings []model.Finding
}

func LoadRun(path string) (RunData, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return RunData{}, fmt.Errorf("read %s: %w", path, err)
	}
	var rr model.RunReport
	if err := json.Unmarshal(b, &rr); err != nil {
		return RunData{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return RunData{Report: rr}, nil
}

func LoadFindings(path string) ([]model.Finding, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var ff []model.Finding
	if err := json.Unmarshal(b, &ff); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return ff, nil
}

func Compare(a, b RunData) CompareResult {
	indexA := map[FindingKey]model.Finding{}
	for _, f := range a.Findings {
		indexA[keyFor(f)] = f
	}
	indexB := map[FindingKey]model.Finding{}
	for _, f := range b.Findings {
		indexB[keyFor(f)] = f
	}

	allKeys := map[FindingKey]bool{}
	for k := range indexA {
		allKeys[k] = true
	}
	for k := range indexB {
		allKeys[k] = true
	}

	var agreed, onlyA, onlyB []FindingPair
	sevMatched, sevMismatched, sevTotal := 0, 0, 0

	sorted := sortedKeys(allKeys)
	for _, k := range sorted {
		fa, inA := indexA[k]
		fb, inB := indexB[k]
		switch {
		case inA && inB:
			pfa, pfb := fa, fb
			agreed = append(agreed, FindingPair{Key: k, RunA: &pfa, RunB: &pfb})
			sevTotal++
			if fa.Severity == fb.Severity {
				sevMatched++
			} else {
				sevMismatched++
			}
		case inA:
			pfa := fa
			onlyA = append(onlyA, FindingPair{Key: k, RunA: &pfa})
		default:
			pfb := fb
			onlyB = append(onlyB, FindingPair{Key: k, RunB: &pfb})
		}
	}

	total := len(allKeys)
	agreeRate := 0.0
	disagreeRate := 0.0
	if total > 0 {
		agreeRate = float64(len(agreed)) / float64(total)
		disagreeRate = float64(len(onlyA)+len(onlyB)) / float64(total)
	}

	return CompareResult{
		RunA:            a.Report.RunID,
		RunB:            b.Report.RunID,
		ModelA:          primaryModel(a.Report),
		ModelB:          primaryModel(b.Report),
		TotalA:          len(a.Findings),
		TotalB:          len(b.Findings),
		Agreed:          agreed,
		OnlyA:           onlyA,
		OnlyB:           onlyB,
		AgreementRate:   agreeRate,
		DisagreementRate: disagreeRate,
		SeverityAlign: SeverityAlignment{
			Matched:    sevMatched,
			Mismatched: sevMismatched,
			Total:      sevTotal,
		},
	}
}

func FormatText(r CompareResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Model Comparison: %s vs %s\n", r.ModelA, r.ModelB)
	fmt.Fprintf(&sb, "  Run A: %s (%d findings)\n", r.RunA, r.TotalA)
	fmt.Fprintf(&sb, "  Run B: %s (%d findings)\n\n", r.RunB, r.TotalB)
	fmt.Fprintf(&sb, "  Agreement:    %d findings (%.1f%%)\n", len(r.Agreed), r.AgreementRate*100)
	fmt.Fprintf(&sb, "  Only in A:    %d findings\n", len(r.OnlyA))
	fmt.Fprintf(&sb, "  Only in B:    %d findings\n", len(r.OnlyB))
	fmt.Fprintf(&sb, "  Disagreement: %.1f%%\n\n", r.DisagreementRate*100)
	fmt.Fprintf(&sb, "  Severity Alignment: %d/%d matched (%.1f%%)\n",
		r.SeverityAlign.Matched, r.SeverityAlign.Total,
		pct(r.SeverityAlign.Matched, r.SeverityAlign.Total))

	if len(r.OnlyA) > 0 {
		fmt.Fprintf(&sb, "\nFindings only in %s:\n", r.ModelA)
		for _, p := range r.OnlyA {
			fmt.Fprintf(&sb, "  [%s] %s — %s\n", p.RunA.Severity, p.Key.DedupKey, p.RunA.Summary)
		}
	}
	if len(r.OnlyB) > 0 {
		fmt.Fprintf(&sb, "\nFindings only in %s:\n", r.ModelB)
		for _, p := range r.OnlyB {
			fmt.Fprintf(&sb, "  [%s] %s — %s\n", p.RunB.Severity, p.Key.DedupKey, p.RunB.Summary)
		}
	}
	if r.SeverityAlign.Mismatched > 0 {
		fmt.Fprintf(&sb, "\nSeverity Mismatches:\n")
		for _, p := range r.Agreed {
			if p.RunA.Severity != p.RunB.Severity {
				fmt.Fprintf(&sb, "  %s: %s (%s) vs %s (%s)\n",
					p.Key.DedupKey, p.RunA.Severity, r.ModelA, p.RunB.Severity, r.ModelB)
			}
		}
	}
	return sb.String()
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d) * 100
}

func sortedKeys(m map[FindingKey]bool) []FindingKey {
	keys := make([]FindingKey, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Lab != keys[j].Lab {
			return keys[i].Lab < keys[j].Lab
		}
		return keys[i].DedupKey < keys[j].DedupKey
	})
	return keys
}
