package diversity

import (
	"fmt"
	"math"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Report struct {
	TotalFindings   int              `json:"total_findings"`
	UniqueClasses   int              `json:"unique_classes"`
	UniqueLocations int              `json:"unique_locations"`
	ClassDist       map[string]int   `json:"class_distribution"`
	SeverityDist    map[string]int   `json:"severity_distribution"`
	ModelDist       map[string]int   `json:"model_distribution"`
	Metrics         Metrics          `json:"metrics"`
}

type Metrics struct {
	ClassEntropy    float64 `json:"class_entropy"`
	SeveritySpread  float64 `json:"severity_spread"`
	LocationCoverage float64 `json:"location_coverage"`
	ModelAgreement  float64 `json:"model_agreement"`
	OverallScore    float64 `json:"overall_score"`
}

const (
	weightClassEntropy    = 0.30
	weightSeveritySpread  = 0.20
	weightLocationCov     = 0.25
	weightModelAgreement  = 0.25
)

func Analyze(findings []model.Finding) Report {
	r := Report{
		TotalFindings: len(findings),
		ClassDist:     make(map[string]int),
		SeverityDist:  make(map[string]int),
		ModelDist:     make(map[string]int),
	}

	if len(findings) == 0 {
		return r
	}

	classes := make(map[string]bool)
	locations := make(map[string]bool)
	models := make(map[string]bool)

	for _, f := range findings {
		cls := string(f.Class)
		r.ClassDist[cls]++
		classes[cls] = true

		sev := string(f.Severity)
		r.SeverityDist[sev]++

		loc := f.Location.Method + " " + f.Location.PathTemplate
		locations[loc] = true

		mdl := f.Model.Model
		if mdl == "" {
			mdl = "unknown"
		}
		r.ModelDist[mdl]++
		models[mdl] = true
	}

	r.UniqueClasses = len(classes)
	r.UniqueLocations = len(locations)

	r.Metrics = computeMetrics(findings, r)
	return r
}

func computeMetrics(findings []model.Finding, r Report) Metrics {
	m := Metrics{}

	m.ClassEntropy = normalizedEntropy(r.ClassDist, len(findings))

	m.SeveritySpread = severitySpread(r.SeverityDist)

	allClasses := len(model.AllClasses)
	if allClasses > 0 {
		m.LocationCoverage = math.Min(1.0, float64(r.UniqueClasses)/float64(allClasses))
	}

	m.ModelAgreement = modelAgreement(findings)

	m.OverallScore = round2(
		weightClassEntropy*m.ClassEntropy +
			weightSeveritySpread*m.SeveritySpread +
			weightLocationCov*m.LocationCoverage +
			weightModelAgreement*m.ModelAgreement,
	)
	m.ClassEntropy = round2(m.ClassEntropy)
	m.SeveritySpread = round2(m.SeveritySpread)
	m.LocationCoverage = round2(m.LocationCoverage)
	m.ModelAgreement = round2(m.ModelAgreement)

	return m
}

func normalizedEntropy(dist map[string]int, total int) float64 {
	if total == 0 || len(dist) <= 1 {
		return 0
	}
	var h float64
	for _, count := range dist {
		p := float64(count) / float64(total)
		if p > 0 {
			h -= p * math.Log2(p)
		}
	}
	maxH := math.Log2(float64(len(dist)))
	if maxH == 0 {
		return 0
	}
	return h / maxH
}

func severitySpread(dist map[string]int) float64 {
	levels := []string{"info", "low", "medium", "high", "critical"}
	present := 0
	for _, l := range levels {
		if dist[l] > 0 {
			present++
		}
	}
	if len(levels) == 0 {
		return 0
	}
	return float64(present) / float64(len(levels))
}

func modelAgreement(findings []model.Finding) float64 {
	byDedup := make(map[string]map[string]bool)
	for _, f := range findings {
		key := f.DedupKey
		if key == "" {
			key = f.ID
		}
		if byDedup[key] == nil {
			byDedup[key] = make(map[string]bool)
		}
		byDedup[key][f.Model.Model] = true
	}

	if len(byDedup) == 0 {
		return 0
	}

	multiModel := 0
	for _, models := range byDedup {
		if len(models) > 1 {
			multiModel++
		}
	}
	return float64(multiModel) / float64(len(byDedup))
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Diversity Report\n")
	b.WriteString(strings.Repeat("=", 40) + "\n")
	fmt.Fprintf(&b, "Total findings:     %d\n", r.TotalFindings)
	fmt.Fprintf(&b, "Unique classes:     %d\n", r.UniqueClasses)
	fmt.Fprintf(&b, "Unique locations:   %d\n", r.UniqueLocations)
	fmt.Fprintf(&b, "Class entropy:      %.2f\n", r.Metrics.ClassEntropy)
	fmt.Fprintf(&b, "Severity spread:    %.2f\n", r.Metrics.SeveritySpread)
	fmt.Fprintf(&b, "Location coverage:  %.2f\n", r.Metrics.LocationCoverage)
	fmt.Fprintf(&b, "Model agreement:    %.2f\n", r.Metrics.ModelAgreement)
	fmt.Fprintf(&b, "Overall score:      %.2f\n\n", r.Metrics.OverallScore)

	if len(r.ClassDist) > 0 {
		b.WriteString("Class distribution:\n")
		for cls, count := range r.ClassDist {
			fmt.Fprintf(&b, "  %-25s %d\n", cls, count)
		}
	}

	if len(r.SeverityDist) > 0 {
		b.WriteString("\nSeverity distribution:\n")
		for sev, count := range r.SeverityDist {
			fmt.Fprintf(&b, "  %-15s %d\n", sev, count)
		}
	}

	if len(r.ModelDist) > 0 {
		b.WriteString("\nModel distribution:\n")
		for mdl, count := range r.ModelDist {
			fmt.Fprintf(&b, "  %-30s %d\n", mdl, count)
		}
	}

	return b.String()
}
