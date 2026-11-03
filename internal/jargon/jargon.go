package jargon

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type JargonDetail struct {
	TotalWords      int     `json:"total_words"`
	JargonWords     int     `json:"jargon_words"`
	AcronymCount    int     `json:"acronym_count"`
	LongWordCount   int     `json:"long_word_count"`
	JargonDensity   float64 `json:"jargon_density"`
	ReadabilityHint string  `json:"readability_hint"`
}

type ModelJargon struct {
	Model       string       `json:"model"`
	JargonScore float64      `json:"jargon_score"`
	Grade       string       `json:"grade"`
	Detail      JargonDetail `json:"detail"`
	SampleCount int          `json:"sample_count"`
	LabCount    int          `json:"lab_count"`
}

type Report struct {
	Models     []ModelJargon `json:"models"`
	BestModel  string        `json:"best_model"`
	WorstModel string        `json:"worst_model"`
	MeanScore  float64       `json:"mean_score"`
}

var technicalTerms = []string{
	"algorithm", "api", "asynchronous", "authentication", "authorization",
	"backend", "bandwidth", "blockchain", "boilerplate", "bytecode",
	"cache", "callback", "cicd", "cli", "cluster", "concurrency",
	"containerization", "coroutine", "cryptographic", "daemon",
	"deadlock", "decoupled", "dependency", "deployment", "deserialization",
	"devops", "distributed", "dns", "dockerfile", "endpoint",
	"ephemeral", "etl", "failover", "frontend", "garbage collection",
	"graphql", "hashmap", "heuristic", "http", "idempotent",
	"immutable", "ingestion", "instantiate", "interoperability", "jit",
	"kubernetes", "latency", "load balancer", "microservice", "middleware",
	"monorepo", "mutex", "namespace", "nginx", "oauth",
	"orchestration", "parallelism", "payload", "polymorphism", "provisioning",
	"proxy", "rbac", "refactor", "regex", "replication",
	"rest", "rollback", "runtime", "scalability", "schema",
	"sdk", "serialization", "sharding", "singleton", "socket",
	"ssl", "stateless", "tcp", "terraform", "throughput",
	"tls", "token", "transpile", "udp", "virtualization",
	"webhook", "websocket",
}

func isAcronym(word string) bool {
	if len(word) < 2 || len(word) > 6 {
		return false
	}
	for _, c := range word {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

func isLongWord(word string) bool {
	return len(word) >= 12
}

func countJargon(text string) (int, int) {
	lower := strings.ToLower(text)
	jargonCount := 0
	for _, term := range technicalTerms {
		jargonCount += strings.Count(lower, term)
	}
	words := strings.Fields(text)
	acronymCount := 0
	for _, w := range words {
		clean := strings.Trim(w, ".,;:!?()[]{}\"'")
		if isAcronym(clean) {
			acronymCount++
		}
	}
	return jargonCount, acronymCount
}

func scoreResponse(text string) (float64, JargonDetail) {
	words := strings.Fields(text)
	totalWords := len(words)
	if totalWords == 0 {
		return 50.0, JargonDetail{ReadabilityHint: "empty"}
	}

	jargonCount, acronymCount := countJargon(text)

	longCount := 0
	for _, w := range words {
		if isLongWord(w) {
			longCount++
		}
	}

	jargonDensity := float64(jargonCount) / float64(totalWords)

	hint := "clear"
	if jargonDensity > 0.15 {
		hint = "heavy jargon"
	} else if jargonDensity > 0.08 {
		hint = "moderate jargon"
	} else if jargonDensity > 0.03 {
		hint = "light jargon"
	}

	detail := JargonDetail{
		TotalWords:      totalWords,
		JargonWords:     jargonCount,
		AcronymCount:    acronymCount,
		LongWordCount:   longCount,
		JargonDensity:   math.Round(jargonDensity*1000) / 1000,
		ReadabilityHint: hint,
	}

	score := 80.0
	score -= jargonDensity * 150.0
	score -= float64(acronymCount) * 3.0
	longRatio := float64(longCount) / float64(totalWords)
	score -= longRatio * 50.0

	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return math.Round(score*10) / 10, detail
}

func scoreToGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

func Analyze(findings []model.Finding) Report {
	if len(findings) == 0 {
		return Report{}
	}

	modelEntries := map[string][]model.Finding{}
	for _, f := range findings {
		mn := f.Model.Model
		if mn == "" {
			mn = "unknown"
		}
		modelEntries[mn] = append(modelEntries[mn], f)
	}

	var models []ModelJargon
	for mn, entries := range modelEntries {
		var totalScore float64
		var totalDetail JargonDetail
		labs := map[string]bool{}

		for _, e := range entries {
			labs[e.Lab] = true
			score, detail := scoreResponse(e.Summary)
			totalScore += score
			totalDetail.TotalWords += detail.TotalWords
			totalDetail.JargonWords += detail.JargonWords
			totalDetail.AcronymCount += detail.AcronymCount
			totalDetail.LongWordCount += detail.LongWordCount
			totalDetail.JargonDensity += detail.JargonDensity
		}

		n := float64(len(entries))
		avgScore := totalScore / n
		totalDetail.JargonDensity = math.Round(totalDetail.JargonDensity/n*1000) / 1000

		if totalDetail.JargonDensity > 0.15 {
			totalDetail.ReadabilityHint = "heavy jargon"
		} else if totalDetail.JargonDensity > 0.08 {
			totalDetail.ReadabilityHint = "moderate jargon"
		} else if totalDetail.JargonDensity > 0.03 {
			totalDetail.ReadabilityHint = "light jargon"
		} else {
			totalDetail.ReadabilityHint = "clear"
		}

		models = append(models, ModelJargon{
			Model:       mn,
			JargonScore: math.Round(avgScore*10) / 10,
			Grade:       scoreToGrade(avgScore),
			Detail:      totalDetail,
			SampleCount: len(entries),
			LabCount:    len(labs),
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].JargonScore > models[j].JargonScore
	})

	var sumScore float64
	for _, m := range models {
		sumScore += m.JargonScore
	}

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		r.MeanScore = math.Round(sumScore/float64(len(models))*10) / 10
	}
	return r
}

func FormatText(r Report) string {
	var b strings.Builder
	b.WriteString("Response Jargon Analysis\n")
	b.WriteString(strings.Repeat("=", 50) + "\n\n")

	if len(r.Models) == 0 {
		b.WriteString("  No jargon data available.\n")
		return b.String()
	}

	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s\n", m.Model)
		fmt.Fprintf(&b, "    Score: %.1f  Grade: %s  Hint: %s\n", m.JargonScore, m.Grade, m.Detail.ReadabilityHint)
		fmt.Fprintf(&b, "    Words: %d  Jargon: %d  Acronyms: %d  Long: %d\n",
			m.Detail.TotalWords, m.Detail.JargonWords,
			m.Detail.AcronymCount, m.Detail.LongWordCount)
		fmt.Fprintf(&b, "    Density: %.3f  Samples: %d  Labs: %d\n\n",
			m.Detail.JargonDensity, m.SampleCount, m.LabCount)
	}

	fmt.Fprintf(&b, "  Best: %s  Worst: %s  Mean: %.1f\n",
		r.BestModel, r.WorstModel, r.MeanScore)

	return b.String()
}
