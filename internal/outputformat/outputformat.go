package outputformat

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type FieldCheck struct {
	Field    string `json:"field"`
	Present  int    `json:"present"`
	Missing  int    `json:"missing"`
	Coverage float64 `json:"coverage"`
}

type ModelCompliance struct {
	Model       string       `json:"model"`
	Total       int          `json:"total"`
	Compliant   int          `json:"compliant"`
	Score       float64      `json:"score"`
	Fields      []FieldCheck `json:"fields"`
	Issues      []string     `json:"issues,omitempty"`
}

type Report struct {
	Models         []ModelCompliance `json:"models"`
	OverallScore   float64           `json:"overall_score"`
	RequiredFields []string          `json:"required_fields"`
}

var defaultRequired = []string{
	"id", "class", "severity", "state", "summary", "location",
}

func Analyze(findings []model.Finding, requiredFields []string) Report {
	if len(requiredFields) == 0 {
		requiredFields = defaultRequired
	}

	byModel := make(map[string][]model.Finding)
	for _, f := range findings {
		byModel[f.Model.Model] = append(byModel[f.Model.Model], f)
	}

	var models []ModelCompliance
	for m, fs := range byModel {
		mc := checkModel(m, fs, requiredFields)
		models = append(models, mc)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Model < models[j].Model
	})

	overall := 0.0
	if len(models) > 0 {
		sum := 0.0
		for _, m := range models {
			sum += m.Score
		}
		overall = sum / float64(len(models))
	}

	return Report{
		Models:         models,
		OverallScore:   round2(overall),
		RequiredFields: requiredFields,
	}
}

func checkModel(modelName string, findings []model.Finding, required []string) ModelCompliance {
	fields := make([]FieldCheck, len(required))
	for i, rf := range required {
		present := 0
		missing := 0
		for _, f := range findings {
			if hasField(f, rf) {
				present++
			} else {
				missing++
			}
		}
		cov := 0.0
		if present+missing > 0 {
			cov = float64(present) / float64(present+missing)
		}
		fields[i] = FieldCheck{
			Field:    rf,
			Present:  present,
			Missing:  missing,
			Coverage: round2(cov),
		}
	}

	compliant := 0
	for _, f := range findings {
		if isCompliant(f, required) {
			compliant++
		}
	}

	score := 0.0
	if len(findings) > 0 {
		score = float64(compliant) / float64(len(findings))
	}

	var issues []string
	for _, fc := range fields {
		if fc.Coverage < 1.0 {
			issues = append(issues, fmt.Sprintf("%s missing in %d/%d findings", fc.Field, fc.Missing, fc.Present+fc.Missing))
		}
	}

	return ModelCompliance{
		Model:     modelName,
		Total:     len(findings),
		Compliant: compliant,
		Score:     round2(score),
		Fields:    fields,
		Issues:    issues,
	}
}

func hasField(f model.Finding, field string) bool {
	switch field {
	case "id":
		return f.ID != ""
	case "class":
		return f.Class != ""
	case "severity":
		return f.Severity != ""
	case "state":
		return f.State != ""
	case "summary":
		return f.Summary != ""
	case "location":
		return f.Location.Method != "" || f.Location.PathTemplate != ""
	case "cwe":
		return f.CWE != ""
	case "evidence":
		return len(f.Evidence) > 0
	case "model":
		return f.Model.Model != ""
	case "dedup_key":
		return f.DedupKey != ""
	case "run_id":
		return f.RunID != ""
	case "prompt_hash":
		return f.PromptHash != ""
	default:
		return false
	}
}

func isCompliant(f model.Finding, required []string) bool {
	for _, r := range required {
		if !hasField(f, r) {
			return false
		}
	}
	return true
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Output Format Compliance Analysis\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No findings to analyze.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "  Models analyzed:      %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Overall compliance:   %.0f%%\n", r.OverallScore*100)
	fmt.Fprintf(&b, "  Required fields:      %s\n\n", strings.Join(r.RequiredFields, ", "))

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %s\n", m.Model)
		fmt.Fprintf(&b, "    Findings: %d  Compliant: %d (%.0f%%)\n",
			m.Total, m.Compliant, m.Score*100)

		for _, fc := range m.Fields {
			status := "OK"
			if fc.Coverage < 1.0 {
				status = fmt.Sprintf("%.0f%%", fc.Coverage*100)
			}
			fmt.Fprintf(&b, "    %-16s %s\n", fc.Field, status)
		}

		if len(m.Issues) > 0 {
			b.WriteString("    Issues:\n")
			for _, issue := range m.Issues {
				fmt.Fprintf(&b, "      - %s\n", issue)
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b) + "\n"
}
