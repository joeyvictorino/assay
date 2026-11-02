package falsepos

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type ClassRate struct {
	Class         string  `json:"class"`
	Total         int     `json:"total"`
	Refuted       int     `json:"refuted"`
	FPRate        float64 `json:"fp_rate"`
	Confidence    float64 `json:"confidence"`
	ConfInterval  [2]float64 `json:"confidence_interval"`
}

type ModelRate struct {
	Model        string      `json:"model"`
	Total        int         `json:"total"`
	Refuted      int         `json:"refuted"`
	FPRate       float64     `json:"fp_rate"`
	ByClass      []ClassRate `json:"by_class"`
}

type Report struct {
	TotalFindings    int         `json:"total_findings"`
	ValidatedCount   int         `json:"validated_count"`
	RefutedCount     int         `json:"refuted_count"`
	OverallFPRate    float64     `json:"overall_fp_rate"`
	ByClass          []ClassRate `json:"by_class"`
	ByModel          []ModelRate `json:"by_model"`
	Recommendation   string      `json:"recommendation"`
}

func wilsonInterval(n, k int, z float64) (float64, float64) {
	if n == 0 {
		return 0, 1
	}
	p := float64(k) / float64(n)
	nf := float64(n)
	denom := 1 + z*z/nf
	centre := (p + z*z/(2*nf)) / denom
	half := z * math.Sqrt((p*(1-p)+z*z/(4*nf))/nf) / denom
	lo := math.Max(0, centre-half)
	hi := math.Min(1, centre+half)
	return math.Round(lo*1000) / 1000, math.Round(hi*1000) / 1000
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{Recommendation: "No findings to analyze"}
	}

	validated := 0
	refuted := 0

	type classAccum struct {
		total   int
		refuted int
	}
	type modelAccum struct {
		total   int
		refuted int
		byClass map[string]*classAccum
	}

	classMap := make(map[string]*classAccum)
	modelMap := make(map[string]*modelAccum)

	for _, f := range findings {
		if f.Validation == nil {
			continue
		}
		if f.Validation.State == model.StateValidated {
			validated++
		}
		if f.Validation.State == model.StateRefuted {
			refuted++
		}
		if f.Validation.State != model.StateValidated && f.Validation.State != model.StateRefuted {
			continue
		}

		cls := string(f.Class)
		ca, ok := classMap[cls]
		if !ok {
			ca = &classAccum{}
			classMap[cls] = ca
		}
		ca.total++
		if f.Validation.State == model.StateRefuted {
			ca.refuted++
		}

		mdl := f.Model.Model
		ma, ok := modelMap[mdl]
		if !ok {
			ma = &modelAccum{byClass: make(map[string]*classAccum)}
			modelMap[mdl] = ma
		}
		ma.total++
		if f.Validation.State == model.StateRefuted {
			ma.refuted++
		}
		mca, ok := ma.byClass[cls]
		if !ok {
			mca = &classAccum{}
			ma.byClass[cls] = mca
		}
		mca.total++
		if f.Validation.State == model.StateRefuted {
			mca.refuted++
		}
	}

	checkedTotal := validated + refuted
	overallFP := 0.0
	if checkedTotal > 0 {
		overallFP = math.Round(float64(refuted)/float64(checkedTotal)*1000) / 1000
	}

	var byClass []ClassRate
	for cls, ca := range classMap {
		rate := 0.0
		if ca.total > 0 {
			rate = math.Round(float64(ca.refuted)/float64(ca.total)*1000) / 1000
		}
		lo, hi := wilsonInterval(ca.total, ca.refuted, 1.96)
		conf := 1.0 - (hi - lo)
		if ca.total < 5 {
			conf *= 0.5
		}
		byClass = append(byClass, ClassRate{
			Class:        cls,
			Total:        ca.total,
			Refuted:      ca.refuted,
			FPRate:       rate,
			Confidence:   math.Round(conf*100) / 100,
			ConfInterval: [2]float64{lo, hi},
		})
	}
	sort.Slice(byClass, func(i, j int) bool {
		if byClass[i].FPRate != byClass[j].FPRate {
			return byClass[i].FPRate > byClass[j].FPRate
		}
		return byClass[i].Class < byClass[j].Class
	})

	var byModel []ModelRate
	for mdl, ma := range modelMap {
		rate := 0.0
		if ma.total > 0 {
			rate = math.Round(float64(ma.refuted)/float64(ma.total)*1000) / 1000
		}
		var mClasses []ClassRate
		for cls, ca := range ma.byClass {
			cr := 0.0
			if ca.total > 0 {
				cr = math.Round(float64(ca.refuted)/float64(ca.total)*1000) / 1000
			}
			lo, hi := wilsonInterval(ca.total, ca.refuted, 1.96)
			mClasses = append(mClasses, ClassRate{
				Class:        cls,
				Total:        ca.total,
				Refuted:      ca.refuted,
				FPRate:       cr,
				Confidence:   math.Round((1.0-(hi-lo))*100) / 100,
				ConfInterval: [2]float64{lo, hi},
			})
		}
		sort.Slice(mClasses, func(i, j int) bool {
			return mClasses[i].FPRate > mClasses[j].FPRate
		})
		byModel = append(byModel, ModelRate{
			Model:   mdl,
			Total:   ma.total,
			Refuted: ma.refuted,
			FPRate:  rate,
			ByClass: mClasses,
		})
	}
	sort.Slice(byModel, func(i, j int) bool {
		return byModel[i].FPRate < byModel[j].FPRate
	})

	rec := fmt.Sprintf("Overall FP rate: %.1f%% (%d/%d checked)", overallFP*100, refuted, checkedTotal)
	if overallFP > 0.3 {
		rec += "; consider tuning high-FP classes"
	}

	return Report{
		TotalFindings:  len(findings),
		ValidatedCount: validated,
		RefutedCount:   refuted,
		OverallFPRate:  overallFP,
		ByClass:        byClass,
		ByModel:        byModel,
		Recommendation: rec,
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("False-Positive Rate Estimation\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	fmt.Fprintf(&b, "  Total findings:  %d\n", r.TotalFindings)
	fmt.Fprintf(&b, "  Validated:       %d\n", r.ValidatedCount)
	fmt.Fprintf(&b, "  Refuted (FP):    %d\n", r.RefutedCount)
	fmt.Fprintf(&b, "  Overall FP rate: %.1f%%\n", r.OverallFPRate*100)
	fmt.Fprintf(&b, "  %s\n\n", r.Recommendation)

	if len(r.ByClass) > 0 {
		b.WriteString("  By Class:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, cr := range r.ByClass {
			fmt.Fprintf(&b, "    %-20s %d/%d FP (%.1f%%) CI=[%.1f%%, %.1f%%]\n",
				cr.Class, cr.Refuted, cr.Total, cr.FPRate*100,
				cr.ConfInterval[0]*100, cr.ConfInterval[1]*100)
		}
		b.WriteString("\n")
	}

	if len(r.ByModel) > 0 {
		b.WriteString("  By Model:\n")
		b.WriteString("  " + strings.Repeat("-", 60) + "\n")
		for _, mr := range r.ByModel {
			fmt.Fprintf(&b, "    %s: %d/%d FP (%.1f%%)\n",
				mr.Model, mr.Refuted, mr.Total, mr.FPRate*100)
		}
		b.WriteString("\n")
	}

	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
