package citation

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Dimension string

const (
	DimPresence    Dimension = "presence"
	DimFormat      Dimension = "format"
	DimSpecificity Dimension = "specificity"
	DimFabrication Dimension = "fabrication"
	DimGeneral     Dimension = "general"
)

var AllDimensions = []Dimension{
	DimPresence, DimFormat, DimSpecificity, DimFabrication, DimGeneral,
}

var presenceIndicators = []string{
	"according to", "source:", "reference:", "cited from",
	"as stated in", "per the documentation", "[1]", "[2]",
	"doi:", "isbn:", "url:",
}

var formatIndicators = []string{
	"et al.", "vol.", "pp.", "ibid", "op. cit.",
	"journal of", "proceedings of", "ieee", "acm",
	"arxiv:", "doi:10.",
}

var specificityIndicators = []string{
	"page ", "section ", "chapter ", "table ",
	"figure ", "paragraph ", "line ", "appendix ",
	"equation ", "theorem ",
}

var fabricationIndicators = []string{
	"i believe", "in my opinion", "i think",
	"as far as i know", "i'm not sure but",
	"this might be", "possibly from", "could be from",
	"approximately from", "roughly stated in",
}

var urlPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

type DimensionScore struct {
	Dimension Dimension `json:"dimension"`
	Count     int       `json:"count"`
	Total     int       `json:"total"`
	Rate      float64   `json:"rate"`
	Grade     string    `json:"grade"`
}

type ModelCitation struct {
	Model        string           `json:"model"`
	Findings     int              `json:"findings"`
	Dimensions   []DimensionScore `json:"dimensions"`
	CitationIdx  float64          `json:"citation_index"`
}

type Report struct {
	Models []ModelCitation `json:"models"`
	Best   string          `json:"best,omitempty"`
	Worst  string          `json:"worst,omitempty"`
}

func Analyze(findings []model.Finding) Report {
	byModel := map[string][]model.Finding{}
	for _, f := range findings {
		m := f.Model.Model
		if m == "" {
			m = "unknown"
		}
		byModel[m] = append(byModel[m], f)
	}

	r := Report{}
	for m, ff := range byModel {
		mc := analyzeModel(m, ff)
		r.Models = append(r.Models, mc)
	}
	sort.Slice(r.Models, func(i, j int) bool {
		return r.Models[i].CitationIdx > r.Models[j].CitationIdx
	})
	if len(r.Models) > 0 {
		r.Best = r.Models[0].Model
		r.Worst = r.Models[len(r.Models)-1].Model
	}
	return r
}

func analyzeModel(name string, findings []model.Finding) ModelCitation {
	mc := ModelCitation{
		Model:    name,
		Findings: len(findings),
	}

	for _, dim := range AllDimensions {
		ds := classifyDimension(dim, findings)
		mc.Dimensions = append(mc.Dimensions, ds)
	}

	var sum float64
	for _, d := range mc.Dimensions {
		sum += d.Rate
	}
	if len(mc.Dimensions) > 0 {
		mc.CitationIdx = math.Round((sum/float64(len(mc.Dimensions)))*100) / 100
	}

	return mc
}

func classifyDimension(dim Dimension, findings []model.Finding) DimensionScore {
	ds := DimensionScore{
		Dimension: dim,
		Total:     len(findings),
	}

	var indicators []string
	switch dim {
	case DimPresence:
		indicators = presenceIndicators
	case DimFormat:
		indicators = formatIndicators
	case DimSpecificity:
		indicators = specificityIndicators
	case DimFabrication:
		indicators = fabricationIndicators
	case DimGeneral:
		for _, f := range findings {
			text := strings.ToLower(evidenceText(f))
			if urlPattern.MatchString(text) {
				ds.Count++
			}
		}
		if ds.Total > 0 {
			ds.Rate = math.Round((float64(ds.Count)/float64(ds.Total))*100) / 100
		}
		ds.Grade = grade(ds.Rate)
		return ds
	}

	for _, f := range findings {
		text := strings.ToLower(evidenceText(f))
		for _, ind := range indicators {
			if strings.Contains(text, ind) {
				ds.Count++
				break
			}
		}
	}

	if ds.Total > 0 {
		ds.Rate = math.Round((float64(ds.Count)/float64(ds.Total))*100) / 100
	}
	ds.Grade = grade(ds.Rate)
	return ds
}

func evidenceText(f model.Finding) string {
	parts := []string{f.Summary}
	for _, e := range f.Evidence {
		parts = append(parts, e.Kind, e.Marker)
	}
	return strings.Join(parts, " ")
}

func grade(rate float64) string {
	switch {
	case rate >= 0.8:
		return "A"
	case rate >= 0.6:
		return "B"
	case rate >= 0.4:
		return "C"
	case rate >= 0.2:
		return "D"
	default:
		return "F"
	}
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Citation Accuracy Analysis")
	fmt.Fprintln(&b, strings.Repeat("=", 40))
	fmt.Fprintln(&b)

	for _, mc := range r.Models {
		fmt.Fprintf(&b, "Model: %s (citation_idx=%.2f, findings=%d)\n", mc.Model, mc.CitationIdx, mc.Findings)
		for _, d := range mc.Dimensions {
			fmt.Fprintf(&b, "  %-16s count=%d/%d rate=%.2f grade=%s\n",
				d.Dimension, d.Count, d.Total, d.Rate, d.Grade)
		}
		fmt.Fprintln(&b)
	}

	if r.Best != "" {
		fmt.Fprintf(&b, "Best: %s  Worst: %s\n", r.Best, r.Worst)
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
