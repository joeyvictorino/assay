package technicaldepth

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type DepthDetail struct {
	TechnicalTerms  int     `json:"technical_terms"`
	CodeReferences  int     `json:"code_references"`
	SpecificValues  int     `json:"specific_values"`
	MechanismWords  int     `json:"mechanism_words"`
	DepthRatio      float64 `json:"depth_ratio"`
	TotalWords      int     `json:"total_words"`
}

type ModelDepth struct {
	Model      string      `json:"model"`
	DepthScore float64     `json:"depth_score"`
	Grade      string      `json:"grade"`
	Detail     DepthDetail `json:"detail"`
	SampleCount int        `json:"sample_count"`
	LabCount    int        `json:"lab_count"`
	DeepCount   int        `json:"deep_count"`
}

type Report struct {
	Models     []ModelDepth `json:"models"`
	BestModel  string       `json:"best_model"`
	WorstModel string       `json:"worst_model"`
	MeanScore  float64      `json:"mean_score"`
}

var technicalTerms = []string{
	"algorithm",
	"implementation",
	"architecture",
	"protocol",
	"encryption",
	"authentication",
	"authorization",
	"vulnerability",
	"exploit",
	"buffer overflow",
	"injection",
	"cross-site",
	"certificate",
	"cryptographic",
	"hash function",
	"sql injection",
	"race condition",
	"deadlock",
	"memory leak",
	"stack trace",
}

var codeReferences = []string{
	"function",
	"method",
	"class",
	"variable",
	"parameter",
	"return value",
	"exception",
	"error code",
	"api endpoint",
	"http status",
	"stack frame",
	"heap",
	"pointer",
	"null reference",
}

var specificValues = []string{
	"cve-",
	"port ",
	"version ",
	"0x",
	"bytes",
	"timeout",
	"millisecond",
	"kilobyte",
	"megabyte",
	"line ",
	"offset",
	"index",
}

var mechanismWords = []string{
	"because",
	"therefore",
	"specifically",
	"in particular",
	"the root cause",
	"this occurs when",
	"the mechanism",
	"under the hood",
	"internally",
	"at the system level",
	"the underlying",
	"this is due to",
}

func countWords(text string) int {
	return len(strings.Fields(text))
}

func countPhrases(text string, phrases []string) int {
	low := strings.ToLower(text)
	n := 0
	for _, p := range phrases {
		idx := 0
		for {
			i := strings.Index(low[idx:], p)
			if i < 0 {
				break
			}
			n++
			idx += i + len(p)
		}
	}
	return n
}

func analyzeSample(summary string) DepthDetail {
	tt := countPhrases(summary, technicalTerms)
	cr := countPhrases(summary, codeReferences)
	sv := countPhrases(summary, specificValues)
	mw := countPhrases(summary, mechanismWords)
	tw := countWords(summary)
	total := tt + cr + sv + mw
	ratio := 0.0
	if tw > 0 {
		ratio = float64(total) / float64(tw)
	}
	return DepthDetail{
		TechnicalTerms: tt,
		CodeReferences: cr,
		SpecificValues: sv,
		MechanismWords: mw,
		DepthRatio:     ratio,
		TotalWords:     tw,
	}
}

func grade(score float64) string {
	switch {
	case score >= 0.95:
		return "A+"
	case score >= 0.90:
		return "A"
	case score >= 0.85:
		return "A-"
	case score >= 0.80:
		return "B+"
	case score >= 0.75:
		return "B"
	case score >= 0.70:
		return "B-"
	case score >= 0.60:
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
		byModel[f.Model.Model] = append(byModel[f.Model.Model], f)
	}

	var models []ModelDepth
	for mid, fs := range byModel {
		labs := map[string]bool{}
		var totalTech, totalCode, totalSpecific, totalMechanism int
		var totalWords int
		deepCount := 0

		for _, f := range fs {
			labs[f.Lab] = true
			d := analyzeSample(f.Summary)
			totalTech += d.TechnicalTerms
			totalCode += d.CodeReferences
			totalSpecific += d.SpecificValues
			totalMechanism += d.MechanismWords
			totalWords += d.TotalWords
			if d.DepthRatio > 0.10 {
				deepCount++
			}
		}

		totalDepth := totalTech + totalCode + totalSpecific + totalMechanism
		ratio := 0.0
		if totalWords > 0 {
			ratio = float64(totalDepth) / float64(totalWords)
		}
		score := math.Min(1.0, ratio*5)

		models = append(models, ModelDepth{
			Model:      mid,
			DepthScore: math.Round(score*1000) / 1000,
			Grade:      grade(score),
			Detail: DepthDetail{
				TechnicalTerms: totalTech,
				CodeReferences: totalCode,
				SpecificValues: totalSpecific,
				MechanismWords: totalMechanism,
				DepthRatio:     math.Round(ratio*1000) / 1000,
				TotalWords:     totalWords,
			},
			SampleCount: len(fs),
			LabCount:    len(labs),
			DeepCount:   deepCount,
		})
	}

	sort.Slice(models, func(i, j int) bool { return models[i].DepthScore > models[j].DepthScore })

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		sum := 0.0
		for _, m := range models {
			sum += m.DepthScore
		}
		r.MeanScore = math.Round(sum/float64(len(models))*1000) / 1000
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Technical Depth Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&b, "  Models evaluated:  %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:        %.3f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:        %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:       %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %s  [%s]  score=%.3f  samples=%d  deep=%d\n",
			m.Model, m.Grade, m.DepthScore, m.SampleCount, m.DeepCount)
		fmt.Fprintf(&b, "    technical=%d  code_refs=%d  specific=%d  mechanism=%d\n",
			m.Detail.TechnicalTerms, m.Detail.CodeReferences, m.Detail.SpecificValues, m.Detail.MechanismWords)
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
