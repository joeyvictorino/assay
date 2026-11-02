package classcov

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ClassEntry struct {
	Class        string   `json:"class"`
	FindingCount int      `json:"finding_count"`
	Models       []string `json:"models"`
	MaxSeverity  string   `json:"max_severity"`
}

type ModelCoverage struct {
	Model       string   `json:"model"`
	Covered     int      `json:"covered"`
	Total       int      `json:"total"`
	CoveragePct float64  `json:"coverage_pct"`
	Classes     []string `json:"classes"`
}

type Report struct {
	TotalClasses    int             `json:"total_classes"`
	CoveredClasses  int             `json:"covered_classes"`
	MissingClasses  []string        `json:"missing_classes"`
	CoveragePct     float64         `json:"coverage_pct"`
	ByClass         []ClassEntry    `json:"by_class"`
	ByModel         []ModelCoverage `json:"by_model"`
	Recommendation  string          `json:"recommendation"`
}

func Analyze(findings []model.Finding) Report {
	totalClasses := len(model.AllClasses)
	if len(findings) == 0 {
		missing := make([]string, len(model.AllClasses))
		for i, c := range model.AllClasses {
			missing[i] = string(c)
		}
		return Report{
			TotalClasses:   totalClasses,
			MissingClasses: missing,
			Recommendation: "No findings to analyze",
		}
	}

	classFindings := make(map[model.Class][]model.Finding)
	classModels := make(map[model.Class]map[string]bool)
	modelClasses := make(map[string]map[model.Class]bool)

	for _, f := range findings {
		classFindings[f.Class] = append(classFindings[f.Class], f)
		if classModels[f.Class] == nil {
			classModels[f.Class] = make(map[string]bool)
		}
		mdl := f.Model.Model
		if mdl == "" {
			mdl = "unknown"
		}
		classModels[f.Class][mdl] = true
		if modelClasses[mdl] == nil {
			modelClasses[mdl] = make(map[model.Class]bool)
		}
		modelClasses[mdl][f.Class] = true
	}

	var covered []ClassEntry
	var missing []string
	for _, cls := range model.AllClasses {
		ff, ok := classFindings[cls]
		if !ok || len(ff) == 0 {
			missing = append(missing, string(cls))
			continue
		}
		maxSev := model.Severity("")
		for _, f := range ff {
			if f.Severity.Rank() > maxSev.Rank() {
				maxSev = f.Severity
			}
		}
		models := make([]string, 0, len(classModels[cls]))
		for m := range classModels[cls] {
			models = append(models, m)
		}
		sort.Strings(models)
		covered = append(covered, ClassEntry{
			Class:        string(cls),
			FindingCount: len(ff),
			Models:       models,
			MaxSeverity:  string(maxSev),
		})
	}

	covPct := 0.0
	if totalClasses > 0 {
		covPct = math.Round(float64(len(covered))/float64(totalClasses)*1000) / 1000
	}

	var byModel []ModelCoverage
	for mdl, classes := range modelClasses {
		cls := make([]string, 0, len(classes))
		for c := range classes {
			cls = append(cls, string(c))
		}
		sort.Strings(cls)
		mPct := 0.0
		if totalClasses > 0 {
			mPct = math.Round(float64(len(cls))/float64(totalClasses)*1000) / 1000
		}
		byModel = append(byModel, ModelCoverage{
			Model:       mdl,
			Covered:     len(cls),
			Total:       totalClasses,
			CoveragePct: mPct,
			Classes:     cls,
		})
	}
	sort.Slice(byModel, func(i, j int) bool {
		return byModel[i].CoveragePct > byModel[j].CoveragePct
	})

	rec := fmt.Sprintf("%d/%d classes covered (%.1f%%)", len(covered), totalClasses, covPct*100)
	if covPct < 0.5 {
		rec += "; low coverage — expand scanning scope"
	}

	return Report{
		TotalClasses:   totalClasses,
		CoveredClasses: len(covered),
		MissingClasses: missing,
		CoveragePct:    covPct,
		ByClass:        covered,
		ByModel:        byModel,
		Recommendation: rec,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Vulnerability Class Coverage\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	fmt.Fprintf(&b, "  Total classes:   %d\n", r.TotalClasses)
	fmt.Fprintf(&b, "  Covered:         %d\n", r.CoveredClasses)
	fmt.Fprintf(&b, "  Missing:         %d\n", len(r.MissingClasses))
	fmt.Fprintf(&b, "  Coverage:        %.1f%%\n", r.CoveragePct*100)
	fmt.Fprintf(&b, "  %s\n\n", r.Recommendation)

	if len(r.ByClass) > 0 {
		b.WriteString("  Covered Classes:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, ce := range r.ByClass {
			fmt.Fprintf(&b, "    %-22s %3d findings  max=%s  models=%s\n",
				ce.Class, ce.FindingCount, ce.MaxSeverity, strings.Join(ce.Models, ","))
		}
		b.WriteString("\n")
	}

	if len(r.MissingClasses) > 0 {
		b.WriteString("  Missing Classes:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, mc := range r.MissingClasses {
			fmt.Fprintf(&b, "    %s\n", mc)
		}
		b.WriteString("\n")
	}

	if len(r.ByModel) > 0 {
		b.WriteString("  By Model:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, mc := range r.ByModel {
			fmt.Fprintf(&b, "    %-20s %d/%d (%.1f%%)\n",
				mc.Model, mc.Covered, mc.Total, mc.CoveragePct*100)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
