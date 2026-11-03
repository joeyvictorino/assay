package relevance

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

var securityTerms = []string{
	"vulnerability",
	"exploit",
	"injection",
	"authentication",
	"authorization",
	"cross-site",
	"xss",
	"csrf",
	"sql injection",
	"buffer overflow",
	"privilege escalation",
	"remote code execution",
	"denial of service",
	"information disclosure",
	"broken access control",
	"insecure deserialization",
	"security misconfiguration",
	"sensitive data exposure",
	"cryptographic",
	"encryption",
}

var technicalTerms = []string{
	"endpoint",
	"parameter",
	"payload",
	"request",
	"response",
	"header",
	"cookie",
	"session",
	"token",
	"api",
	"input validation",
	"sanitization",
	"encoding",
	"certificate",
	"tls",
	"ssl",
	"port",
	"protocol",
	"configuration",
	"permission",
}

var actionTerms = []string{
	"remediation",
	"mitigation",
	"patch",
	"fix",
	"update",
	"upgrade",
	"configure",
	"restrict",
	"validate",
	"sanitize",
	"encrypt",
	"authenticate",
	"authorize",
	"filter",
	"block",
	"disable",
	"remove",
	"implement",
	"enforce",
	"monitor",
}

var digressionMarkers = []string{
	"unrelated",
	"off topic",
	"by the way",
	"on a different note",
	"speaking of which",
	"incidentally",
	"as an aside",
	"tangentially",
	"not directly related",
	"while we are at it",
	"fun fact",
	"interesting aside",
	"side note",
	"random thought",
	"just to mention",
	"on another note",
	"changing the subject",
	"before i forget",
	"this reminds me",
	"apropos of nothing",
}

type Detail struct {
	SecurityCount   int     `json:"security_count"`
	TechnicalCount  int     `json:"technical_count"`
	ActionCount     int     `json:"action_count"`
	DigressionCount int     `json:"digression_count"`
	TotalWords      int     `json:"total_words"`
	RelevanceRatio  float64 `json:"relevance_ratio"`
}

type ModelRelevance struct {
	Model       string  `json:"model"`
	Score       float64 `json:"score"`
	Grade       string  `json:"grade"`
	Detail      Detail  `json:"detail"`
	SampleCount int     `json:"sample_count"`
}

type Report struct {
	Models     []ModelRelevance `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

func countWords(text string) int {
	return len(strings.Fields(text))
}

func countPhrases(text string, phrases []string) int {
	n := 0
	for _, p := range phrases {
		idx := 0
		for {
			i := strings.Index(text[idx:], p)
			if i < 0 {
				break
			}
			n++
			idx += i + len(p)
		}
	}
	return n
}

func analyzeSample(summary string) Detail {
	lower := strings.ToLower(summary)
	words := countWords(summary)
	security := countPhrases(lower, securityTerms)
	technical := countPhrases(lower, technicalTerms)
	action := countPhrases(lower, actionTerms)
	digression := countPhrases(lower, digressionMarkers)
	relevant := security + technical + action
	ratio := 0.0
	if words > 0 {
		ratio = float64(relevant) / float64(words)
		if digression > 0 {
			ratio = ratio * math.Max(0.0, 1.0-float64(digression)*0.15)
		}
	}
	return Detail{
		SecurityCount:   security,
		TechnicalCount:  technical,
		ActionCount:     action,
		DigressionCount: digression,
		TotalWords:      words,
		RelevanceRatio:  ratio,
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
	case score >= 0.65:
		return "C+"
	case score >= 0.60:
		return "C"
	case score >= 0.55:
		return "C-"
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

	var models []ModelRelevance
	for name, ff := range byModel {
		var totalSecurity, totalTechnical, totalAction, totalDigression, totalWords int
		for _, f := range ff {
			d := analyzeSample(f.Summary)
			totalSecurity += d.SecurityCount
			totalTechnical += d.TechnicalCount
			totalAction += d.ActionCount
			totalDigression += d.DigressionCount
			totalWords += d.TotalWords
		}
		relevant := totalSecurity + totalTechnical + totalAction
		ratio := 0.0
		if totalWords > 0 {
			ratio = float64(relevant) / float64(totalWords)
			if totalDigression > 0 {
				ratio = ratio * math.Max(0.0, 1.0-float64(totalDigression)*0.15)
			}
		}
		score := math.Min(1.0, ratio*5)
		models = append(models, ModelRelevance{
			Model:       name,
			Score:       score,
			Grade:       grade(score),
			SampleCount: len(ff),
			Detail: Detail{
				SecurityCount:   totalSecurity,
				TechnicalCount:  totalTechnical,
				ActionCount:     totalAction,
				DigressionCount: totalDigression,
				TotalWords:      totalWords,
				RelevanceRatio:  ratio,
			},
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Score > models[j].Score
	})

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		sum := 0.0
		for _, m := range models {
			sum += m.Score
		}
		r.MeanScore = sum / float64(len(models))
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Relevance Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No models analyzed.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.2f  Grade: %s  Samples: %d\n", m.Score, m.Grade, m.SampleCount)
		fmt.Fprintf(&b, "    Security: %d  Technical: %d  Action: %d  Digression: %d\n",
			m.Detail.SecurityCount, m.Detail.TechnicalCount, m.Detail.ActionCount, m.Detail.DigressionCount)
		fmt.Fprintf(&b, "    Words: %d  Relevance ratio: %.4f\n\n", m.Detail.TotalWords, m.Detail.RelevanceRatio)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.2f\n", r.BestModel, r.WorstModel, r.MeanScore)
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
