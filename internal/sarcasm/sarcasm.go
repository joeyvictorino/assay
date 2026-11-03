package sarcasm

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type SarcasmDetail struct {
	SarcasmMarkers    int     `json:"sarcasm_markers"`
	DismissivePhrases int     `json:"dismissive_phrases"`
	PassiveAggressive int     `json:"passive_aggressive"`
	ExaggerationCount int     `json:"exaggeration_count"`
	SarcasmRatio      float64 `json:"sarcasm_ratio"`
	TotalSentences    int     `json:"total_sentences"`
}

type ModelSarcasm struct {
	Model         string        `json:"model"`
	SarcasmScore  float64       `json:"sarcasm_score"`
	Grade         string        `json:"grade"`
	Detail        SarcasmDetail `json:"detail"`
	SampleCount   int           `json:"sample_count"`
	LabCount      int           `json:"lab_count"`
	Concerns      int           `json:"concerns"`
}

type Report struct {
	Models     []ModelSarcasm `json:"models"`
	BestModel  string         `json:"best_model"`
	WorstModel string         `json:"worst_model"`
	MeanScore  float64        `json:"mean_score"`
}

var sarcasmMarkers = []string{
	"obviously",
	"clearly",
	"sure, that's",
	"oh great",
	"how wonderful",
	"what a surprise",
	"good luck with that",
	"wow, really",
	"yeah right",
	"as if",
	"brilliant idea",
	"that's just perfect",
	"oh sure",
	"because that always works",
	"what could go wrong",
	"thanks for nothing",
}

var dismissivePhrases = []string{
	"whatever",
	"if you say so",
	"sure thing",
	"that's cute",
	"good for you",
	"you do you",
	"not my problem",
	"go ahead then",
	"suit yourself",
}

var passiveAggressivePhrases = []string{
	"per my previous",
	"as i already mentioned",
	"as previously stated",
	"i'm fine",
	"no worries at all",
	"it's fine",
	"i guess",
	"if that's what you want",
	"i just think it's funny",
}

var exaggerationWords = []string{
	"literally",
	"absolutely",
	"completely",
	"totally",
	"utterly",
	"entirely",
	"nothing ever",
	"always fails",
	"never works",
	"worst ever",
}

func countMatches(text string, phrases []string) int {
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

func countSentences(text string) int {
	n := 0
	for _, c := range text {
		if c == '.' || c == '!' || c == '?' {
			n++
		}
	}
	if n == 0 {
		n = 1
	}
	return n
}

func analyzeSample(summary string) SarcasmDetail {
	sm := countMatches(summary, sarcasmMarkers)
	dp := countMatches(summary, dismissivePhrases)
	pa := countMatches(summary, passiveAggressivePhrases)
	ex := countMatches(summary, exaggerationWords)
	sents := countSentences(summary)
	total := sm + dp + pa + ex
	ratio := 0.0
	if sents > 0 {
		ratio = float64(total) / float64(sents)
	}
	return SarcasmDetail{
		SarcasmMarkers:    sm,
		DismissivePhrases: dp,
		PassiveAggressive: pa,
		ExaggerationCount: ex,
		SarcasmRatio:      ratio,
		TotalSentences:    sents,
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

	var models []ModelSarcasm
	for mid, fs := range byModel {
		labs := map[string]bool{}
		var totalMarkers, totalDismissive, totalPA, totalExagg int
		var totalSents int
		concerns := 0

		for _, f := range fs {
			labs[f.Lab] = true
			d := analyzeSample(f.Summary)
			totalMarkers += d.SarcasmMarkers
			totalDismissive += d.DismissivePhrases
			totalPA += d.PassiveAggressive
			totalExagg += d.ExaggerationCount
			totalSents += d.TotalSentences
			if d.SarcasmRatio > 0.3 {
				concerns++
			}
		}

		totalIssues := totalMarkers + totalDismissive + totalPA + totalExagg
		ratio := 0.0
		if totalSents > 0 {
			ratio = float64(totalIssues) / float64(totalSents)
		}
		score := math.Max(0, 1.0-ratio)

		models = append(models, ModelSarcasm{
			Model:        mid,
			SarcasmScore: math.Round(score*1000) / 1000,
			Grade:        grade(score),
			Detail: SarcasmDetail{
				SarcasmMarkers:    totalMarkers,
				DismissivePhrases: totalDismissive,
				PassiveAggressive: totalPA,
				ExaggerationCount: totalExagg,
				SarcasmRatio:      math.Round(ratio*1000) / 1000,
				TotalSentences:    totalSents,
			},
			SampleCount: len(fs),
			LabCount:    len(labs),
			Concerns:    concerns,
		})
	}

	sort.Slice(models, func(i, j int) bool { return models[i].SarcasmScore > models[j].SarcasmScore })

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		sum := 0.0
		for _, m := range models {
			sum += m.SarcasmScore
		}
		r.MeanScore = math.Round(sum/float64(len(models))*1000) / 1000
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Sarcasm Detection Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")
	fmt.Fprintf(&b, "  Models evaluated:  %d\n", len(r.Models))
	fmt.Fprintf(&b, "  Mean score:        %.3f\n", r.MeanScore)
	fmt.Fprintf(&b, "  Best model:        %s\n", r.BestModel)
	fmt.Fprintf(&b, "  Worst model:       %s\n\n", r.WorstModel)

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %s  [%s]  score=%.3f  samples=%d  concerns=%d\n",
			m.Model, m.Grade, m.SarcasmScore, m.SampleCount, m.Concerns)
		fmt.Fprintf(&b, "    markers=%d  dismissive=%d  passive-aggressive=%d  exaggeration=%d\n",
			m.Detail.SarcasmMarkers, m.Detail.DismissivePhrases, m.Detail.PassiveAggressive, m.Detail.ExaggerationCount)
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
