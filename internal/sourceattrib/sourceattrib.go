package sourceattrib

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type AttributionDetail struct {
	WithEvidence    int     `json:"with_evidence"`
	WithoutEvidence int     `json:"without_evidence"`
	WithToolCalls   int     `json:"with_tool_calls"`
	WithoutToolCalls int    `json:"without_tool_calls"`
	Reconciled      int     `json:"reconciled"`
	Unreconciled    int     `json:"unreconciled"`
	EvidenceRatio   float64 `json:"evidence_ratio"`
	ToolCallRatio   float64 `json:"tool_call_ratio"`
	ReconcileRatio  float64 `json:"reconcile_ratio"`
}

type ModelAttribution struct {
	Model            string            `json:"model"`
	AttributionScore float64           `json:"attribution_score"`
	Grade            string            `json:"grade"`
	Detail           AttributionDetail `json:"detail"`
	SampleCount      int               `json:"sample_count"`
	LabCount         int               `json:"lab_count"`
}

type Report struct {
	Models         []ModelAttribution `json:"models"`
	BestAttributor string             `json:"best_attributor"`
	WorstAttributor string            `json:"worst_attributor"`
	MeanScore      float64            `json:"mean_score"`
}

func grade(score float64) string {
	switch {
	case score >= 0.95:
		return "A"
	case score >= 0.85:
		return "B"
	case score >= 0.70:
		return "C"
	case score >= 0.50:
		return "D"
	default:
		return "F"
	}
}

func Analyze(findings []model.Finding) Report {
	byModel := map[string][]model.Finding{}
	for _, f := range findings {
		key := f.Model.Model
		if key == "" {
			key = f.Model.Provider + "/" + f.Model.Agent
		}
		byModel[key] = append(byModel[key], f)
	}

	var models []ModelAttribution
	for name, ff := range byModel {
		m := analyzeModel(name, ff)
		models = append(models, m)
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].AttributionScore > models[j].AttributionScore
	})

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestAttributor = models[0].Model
		r.WorstAttributor = models[len(models)-1].Model
		sum := 0.0
		for _, m := range models {
			sum += m.AttributionScore
		}
		r.MeanScore = sum / float64(len(models))
	}
	return r
}

func analyzeModel(name string, findings []model.Finding) ModelAttribution {
	labs := map[string]bool{}
	detail := AttributionDetail{}

	for _, f := range findings {
		labs[f.Lab] = true

		if len(f.Evidence) > 0 {
			detail.WithEvidence++
		} else {
			detail.WithoutEvidence++
		}

		if len(f.ToolCallIDs) > 0 {
			detail.WithToolCalls++
		} else {
			detail.WithoutToolCalls++
		}

		if f.Reconcile.Checked && f.Reconcile.Match {
			detail.Reconciled++
		} else if f.Reconcile.Checked {
			detail.Unreconciled++
		}
	}

	total := float64(len(findings))
	if total > 0 {
		detail.EvidenceRatio = float64(detail.WithEvidence) / total
		detail.ToolCallRatio = float64(detail.WithToolCalls) / total
		checked := detail.Reconciled + detail.Unreconciled
		if checked > 0 {
			detail.ReconcileRatio = float64(detail.Reconciled) / float64(checked)
		}
	}

	score := 0.0
	if total > 0 {
		score = detail.EvidenceRatio*0.4 + detail.ToolCallRatio*0.3 + detail.ReconcileRatio*0.3
	}
	score = math.Round(score*100) / 100

	return ModelAttribution{
		Model:            name,
		AttributionScore: score,
		Grade:            grade(score),
		Detail:           detail,
		SampleCount:      len(findings),
		LabCount:         len(labs),
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Source Attribution Analysis (%d models)\n\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:        %.2f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best attributor:   %s\n", r.BestAttributor)
	fmt.Fprintf(&b, "  Worst attributor:  %s\n\n", r.WorstAttributor)
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %-30s [%s] score=%.2f  evidence=%d/%d  toolcalls=%d/%d  reconciled=%d/%d\n",
			m.Model, m.Grade, m.AttributionScore,
			m.Detail.WithEvidence, m.SampleCount,
			m.Detail.WithToolCalls, m.SampleCount,
			m.Detail.Reconciled, m.Detail.Reconciled+m.Detail.Unreconciled)
	}
	return b.String()
}

func FormatJSON(r Report) string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b) + "\n"
}
