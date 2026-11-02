package classcov

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ClassInfo struct {
	Class      string   `json:"class"`
	Models     []string `json:"models"`
	ModelCount int      `json:"model_count"`
	Total      int      `json:"total"`
	CoverageBy float64  `json:"coverage_by_pct"`
}

type ModelCoverage struct {
	Model        string   `json:"model"`
	Classes      []string `json:"classes"`
	ClassCount   int      `json:"class_count"`
	TotalClasses int      `json:"total_classes"`
	CoveragePct  float64  `json:"coverage_pct"`
}

type Report struct {
	TotalFindings  int             `json:"total_findings"`
	TotalModels    int             `json:"total_models"`
	TotalClasses   int             `json:"total_classes"`
	ByClass        []ClassInfo     `json:"by_class"`
	ByModel        []ModelCoverage `json:"by_model"`
	UndetectedClasses []string     `json:"undetected_classes"`
	Recommendation string          `json:"recommendation"`
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{Recommendation: "No findings to analyze"}
	}

	classModels := make(map[string]map[string]bool)
	classCounts := make(map[string]int)
	modelClasses := make(map[string]map[string]bool)
	allModels := make(map[string]bool)

	for _, f := range findings {
		cls := string(f.Class)
		mdl := f.Model.Model
		allModels[mdl] = true

		if classModels[cls] == nil {
			classModels[cls] = make(map[string]bool)
		}
		classModels[cls][mdl] = true
		classCounts[cls]++

		if modelClasses[mdl] == nil {
			modelClasses[mdl] = make(map[string]bool)
		}
		modelClasses[mdl][cls] = true
	}

	totalModels := len(allModels)
	allClassCount := len(model.AllClasses)

	var byClass []ClassInfo
	detectedClasses := make(map[string]bool)
	for cls, models := range classModels {
		detectedClasses[cls] = true
		var modelList []string
		for m := range models {
			modelList = append(modelList, m)
		}
		sort.Strings(modelList)
		covBy := 0.0
		if totalModels > 0 {
			covBy = float64(len(models)) / float64(totalModels) * 100
		}
		byClass = append(byClass, ClassInfo{
			Class:      cls,
			Models:     modelList,
			ModelCount: len(models),
			Total:      classCounts[cls],
			CoverageBy: float64(int(covBy*10)) / 10,
		})
	}
	sort.Slice(byClass, func(i, j int) bool {
		if byClass[i].ModelCount != byClass[j].ModelCount {
			return byClass[i].ModelCount > byClass[j].ModelCount
		}
		return byClass[i].Class < byClass[j].Class
	})

	var byModel []ModelCoverage
	for mdl, classes := range modelClasses {
		var classList []string
		for c := range classes {
			classList = append(classList, c)
		}
		sort.Strings(classList)
		covPct := 0.0
		if allClassCount > 0 {
			covPct = float64(len(classes)) / float64(allClassCount) * 100
		}
		byModel = append(byModel, ModelCoverage{
			Model:        mdl,
			Classes:      classList,
			ClassCount:   len(classes),
			TotalClasses: allClassCount,
			CoveragePct:  float64(int(covPct*10)) / 10,
		})
	}
	sort.Slice(byModel, func(i, j int) bool {
		if byModel[i].CoveragePct != byModel[j].CoveragePct {
			return byModel[i].CoveragePct > byModel[j].CoveragePct
		}
		return byModel[i].Model < byModel[j].Model
	})

	var undetected []string
	for _, c := range model.AllClasses {
		if !detectedClasses[string(c)] {
			undetected = append(undetected, string(c))
		}
	}

	detectedCount := len(detectedClasses)
	rec := fmt.Sprintf("%d/%d classes detected (%.0f%% coverage)", detectedCount, allClassCount,
		float64(detectedCount)/float64(allClassCount)*100)
	if len(undetected) > 0 && len(undetected) <= 5 {
		rec += fmt.Sprintf("; missing: %s", strings.Join(undetected, ", "))
	}

	return Report{
		TotalFindings:     len(findings),
		TotalModels:       totalModels,
		TotalClasses:      detectedCount,
		ByClass:           byClass,
		ByModel:           byModel,
		UndetectedClasses: undetected,
		Recommendation:    rec,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Vulnerability Class Coverage\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	fmt.Fprintf(&b, "  Total findings:    %d\n", r.TotalFindings)
	fmt.Fprintf(&b, "  Total models:      %d\n", r.TotalModels)
	fmt.Fprintf(&b, "  Classes detected:  %d\n", r.TotalClasses)
	fmt.Fprintf(&b, "  %s\n\n", r.Recommendation)

	if len(r.ByClass) > 0 {
		b.WriteString("  Class Coverage:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, ci := range r.ByClass {
			fmt.Fprintf(&b, "    %-20s %d findings, %d models (%.1f%%)\n",
				ci.Class, ci.Total, ci.ModelCount, ci.CoverageBy)
		}
		b.WriteString("\n")
	}

	if len(r.ByModel) > 0 {
		b.WriteString("  Model Coverage:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, mc := range r.ByModel {
			fmt.Fprintf(&b, "    %-20s %d/%d classes (%.1f%%)\n",
				mc.Model, mc.ClassCount, mc.TotalClasses, mc.CoveragePct)
		}
		b.WriteString("\n")
	}

	if len(r.UndetectedClasses) > 0 {
		b.WriteString("  Undetected Classes:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, c := range r.UndetectedClasses {
			fmt.Fprintf(&b, "    - %s\n", c)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
