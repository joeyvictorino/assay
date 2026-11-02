package multimodal

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Dimension string

const (
	DimModalityCoverage  Dimension = "modality_coverage"
	DimCrossModalConsist Dimension = "cross_modal_consistency"
	DimTextImageAlign    Dimension = "text_image_alignment"
	DimCodeTextCoherence Dimension = "code_text_coherence"
	DimGeneral           Dimension = "general"
)

var AllDimensions = []Dimension{
	DimModalityCoverage, DimCrossModalConsist, DimTextImageAlign,
	DimCodeTextCoherence, DimGeneral,
}

var modalityIndicators = map[string][]string{
	"image": {"image", "picture", "photo", "screenshot", "diagram",
		"figure", "visual", "illustration", "graphic", "chart"},
	"code": {"```", "code block", "snippet", "function", "class ",
		"def ", "import ", "return ", "const ", "var "},
	"table": {"| ", "table", "row", "column", "header", "cell"},
	"audio": {"audio", "speech", "voice", "transcript", "spoken"},
	"video": {"video", "clip", "frame", "scene", "footage"},
}

var inconsistencyPatterns = []string{
	"however the image shows", "contrary to the text",
	"does not match", "inconsistent with", "contradicts",
	"the code suggests otherwise", "misaligned with",
	"but the diagram", "despite the visual",
}

type DimensionScore struct {
	Dimension Dimension `json:"dimension"`
	Score     float64   `json:"score"`
	Details   string    `json:"details"`
}

type ModalityBreakdown struct {
	Modality string  `json:"modality"`
	Count    int     `json:"count"`
	Fraction float64 `json:"fraction"`
}

type Report struct {
	TotalFindings     int                 `json:"total_findings"`
	ModalityBreakdown []ModalityBreakdown `json:"modality_breakdown"`
	DimensionScores   []DimensionScore    `json:"dimension_scores"`
	OverallScore      float64             `json:"overall_score"`
	MultiModalCount   int                 `json:"multi_modal_count"`
	SingleModalCount  int                 `json:"single_modal_count"`
}

func Analyze(findings []model.Finding) Report {
	r := Report{TotalFindings: len(findings)}
	if len(findings) == 0 {
		return r
	}

	modalityCounts := map[string]int{}
	multiModal := 0
	singleModal := 0

	for _, f := range findings {
		text := findingText(f)
		modalities := detectModalities(text)
		if len(modalities) > 1 {
			multiModal++
		} else {
			singleModal++
		}
		for _, m := range modalities {
			modalityCounts[m]++
		}
	}

	r.MultiModalCount = multiModal
	r.SingleModalCount = singleModal

	for mod, cnt := range modalityCounts {
		r.ModalityBreakdown = append(r.ModalityBreakdown, ModalityBreakdown{
			Modality: mod,
			Count:    cnt,
			Fraction: float64(cnt) / float64(len(findings)),
		})
	}

	for _, dim := range AllDimensions {
		ds := scoreDimension(dim, findings)
		r.DimensionScores = append(r.DimensionScores, ds)
	}

	sum := 0.0
	for _, ds := range r.DimensionScores {
		sum += ds.Score
	}
	r.OverallScore = math.Round(sum/float64(len(r.DimensionScores))*100) / 100

	return r
}

func findingText(f model.Finding) string {
	parts := []string{f.Summary}
	for _, e := range f.Evidence {
		parts = append(parts, e.Kind, e.Marker)
	}
	return strings.Join(parts, " ")
}

func detectModalities(text string) []string {
	lower := strings.ToLower(text)
	var found []string
	seen := map[string]bool{}
	always := "text"
	seen[always] = true
	found = append(found, always)
	for mod, indicators := range modalityIndicators {
		if seen[mod] {
			continue
		}
		for _, ind := range indicators {
			if strings.Contains(lower, ind) {
				found = append(found, mod)
				seen[mod] = true
				break
			}
		}
	}
	return found
}

func scoreDimension(dim Dimension, findings []model.Finding) DimensionScore {
	ds := DimensionScore{Dimension: dim}

	switch dim {
	case DimModalityCoverage:
		modalities := map[string]bool{}
		for _, f := range findings {
			for _, m := range detectModalities(findingText(f)) {
				modalities[m] = true
			}
		}
		coverage := float64(len(modalities)) / float64(len(modalityIndicators)+1)
		ds.Score = math.Min(1.0, coverage)
		ds.Details = fmt.Sprintf("%d distinct modalities detected across findings", len(modalities))

	case DimCrossModalConsist:
		inconsistent := 0
		for _, f := range findings {
			text := strings.ToLower(findingText(f))
			for _, pat := range inconsistencyPatterns {
				if strings.Contains(text, pat) {
					inconsistent++
					break
				}
			}
		}
		if len(findings) > 0 {
			ds.Score = 1.0 - float64(inconsistent)/float64(len(findings))
		}
		ds.Details = fmt.Sprintf("%d/%d findings show cross-modal inconsistencies", inconsistent, len(findings))

	case DimTextImageAlign:
		aligned := 0
		imageFindings := 0
		for _, f := range findings {
			text := findingText(f)
			mods := detectModalities(text)
			hasImage := false
			for _, m := range mods {
				if m == "image" {
					hasImage = true
					break
				}
			}
			if hasImage {
				imageFindings++
				lower := strings.ToLower(text)
				if !containsAny(lower, inconsistencyPatterns) {
					aligned++
				}
			}
		}
		if imageFindings > 0 {
			ds.Score = float64(aligned) / float64(imageFindings)
		} else {
			ds.Score = 1.0
		}
		ds.Details = fmt.Sprintf("%d/%d image findings aligned with text", aligned, imageFindings)

	case DimCodeTextCoherence:
		coherent := 0
		codeFindings := 0
		for _, f := range findings {
			text := findingText(f)
			mods := detectModalities(text)
			hasCode := false
			for _, m := range mods {
				if m == "code" {
					hasCode = true
					break
				}
			}
			if hasCode {
				codeFindings++
				lower := strings.ToLower(text)
				if !containsAny(lower, inconsistencyPatterns) {
					coherent++
				}
			}
		}
		if codeFindings > 0 {
			ds.Score = float64(coherent) / float64(codeFindings)
		} else {
			ds.Score = 1.0
		}
		ds.Details = fmt.Sprintf("%d/%d code findings coherent with text", coherent, codeFindings)

	case DimGeneral:
		if len(findings) == 0 {
			ds.Score = 1.0
			ds.Details = "no findings to analyze"
			break
		}
		multiCount := 0
		for _, f := range findings {
			mods := detectModalities(findingText(f))
			if len(mods) > 1 {
				multiCount++
			}
		}
		ds.Score = float64(multiCount) / float64(len(findings))
		ds.Details = fmt.Sprintf("%d/%d findings are multi-modal", multiCount, len(findings))
	}

	ds.Score = math.Round(ds.Score*100) / 100
	return ds
}

func containsAny(text string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Multi-Modal Response Consistency Analysis\n")
	fmt.Fprintf(&b, "=========================================\n\n")
	fmt.Fprintf(&b, "Total findings: %d (multi-modal: %d, single-modal: %d)\n\n",
		r.TotalFindings, r.MultiModalCount, r.SingleModalCount)

	if len(r.ModalityBreakdown) > 0 {
		fmt.Fprintf(&b, "Modality Breakdown:\n")
		for _, mb := range r.ModalityBreakdown {
			fmt.Fprintf(&b, "  %-10s  %d (%.0f%%)\n", mb.Modality, mb.Count, mb.Fraction*100)
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintf(&b, "Dimension Scores:\n")
	for _, ds := range r.DimensionScores {
		fmt.Fprintf(&b, "  %-30s  %.2f  %s\n", ds.Dimension, ds.Score, ds.Details)
	}
	fmt.Fprintf(&b, "\nOverall Score: %.2f\n", r.OverallScore)
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
