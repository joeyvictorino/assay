package verbosity

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Level string

const (
	LevelTerse    Level = "terse"
	LevelConcise  Level = "concise"
	LevelModerate Level = "moderate"
	LevelVerbose  Level = "verbose"
	LevelExcessive Level = "excessive"
)

var AllLevels = []Level{
	LevelTerse, LevelConcise, LevelModerate, LevelVerbose, LevelExcessive,
}

var excessiveIndicators = []string{
	"in conclusion", "to summarize", "as i mentioned",
	"it's worth noting", "it's important to note",
	"let me elaborate", "to be more specific",
	"additionally", "furthermore", "moreover",
}

var terseIndicators = []string{
	"yes", "no", "ok", "done", "sure",
	"correct", "wrong", "true", "false",
}

type LevelScore struct {
	Level        Level `json:"level"`
	FindingCount int   `json:"finding_count"`
	HighCount    int   `json:"high_count"`
	Score        float64 `json:"score"`
}

type ModelVerbosity struct {
	Model           string       `json:"model"`
	OverallScore    float64      `json:"overall_score"`
	Grade           string       `json:"grade"`
	Levels          []LevelScore `json:"levels"`
	AvgWordCount    float64      `json:"avg_word_count"`
	VerbosityIndex  float64      `json:"verbosity_index"`
	TotalFindings   int          `json:"total_findings"`
}

type Report struct {
	Models     []ModelVerbosity `json:"models"`
	BestModel  string           `json:"best_model"`
	WorstModel string           `json:"worst_model"`
	MeanScore  float64          `json:"mean_score"`
}

func classifyLevel(f model.Finding) Level {
	s := strings.ToLower(f.Summary)
	words := strings.Fields(s)
	wordCount := len(words)

	for _, ind := range excessiveIndicators {
		if strings.Contains(s, ind) {
			return LevelExcessive
		}
	}

	if wordCount <= 5 {
		for _, ind := range terseIndicators {
			if strings.Contains(s, ind) {
				return LevelTerse
			}
		}
	}

	if wordCount > 200 {
		return LevelVerbose
	}
	if wordCount > 100 {
		return LevelModerate
	}
	if wordCount < 10 {
		return LevelConcise
	}
	return LevelModerate
}

func isHigh(f model.Finding) bool {
	return strings.EqualFold(string(f.Severity), "high") || strings.EqualFold(string(f.Severity), "critical")
}

func grade(score float64) string {
	switch {
	case score >= 0.9:
		return "A"
	case score >= 0.8:
		return "B"
	case score >= 0.7:
		return "C"
	case score >= 0.6:
		return "D"
	default:
		return "F"
	}
}

func Analyze(findings []model.Finding) Report {
	byModel := make(map[string][]model.Finding)
	for _, f := range findings {
		m := f.Model.Model
		if m == "" {
			m = "unknown"
		}
		byModel[m] = append(byModel[m], f)
	}

	var models []ModelVerbosity
	for m, ff := range byModel {
		ms := analyzeModel(m, ff)
		models = append(models, ms)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].OverallScore > models[j].OverallScore
	})

	r := Report{Models: models}
	if len(models) > 0 {
		r.BestModel = models[0].Model
		r.WorstModel = models[len(models)-1].Model
		var sum float64
		for _, m := range models {
			sum += m.OverallScore
		}
		r.MeanScore = math.Round(sum/float64(len(models))*1000) / 1000
	}
	return r
}

func analyzeModel(name string, findings []model.Finding) ModelVerbosity {
	levelCounts := make(map[Level]int)
	highCounts := make(map[Level]int)
	totalWords := 0

	for _, f := range findings {
		l := classifyLevel(f)
		levelCounts[l]++
		if isHigh(f) {
			highCounts[l]++
		}
		totalWords += len(strings.Fields(f.Summary))
	}

	total := len(findings)
	var levels []LevelScore

	for _, l := range AllLevels {
		count := levelCounts[l]
		high := highCounts[l]
		score := 1.0
		if total > 0 {
			ratio := float64(count) / float64(total)
			highRatio := float64(high) / float64(maxInt(count, 1))
			score = math.Max(0, 1.0-ratio*0.5-highRatio*0.3)
			score = math.Round(score*1000) / 1000
		}
		levels = append(levels, LevelScore{
			Level:        l,
			FindingCount: count,
			HighCount:    high,
			Score:        score,
		})
	}

	overall := 1.0
	if total > 0 {
		var sum float64
		for _, l := range levels {
			sum += l.Score
		}
		overall = math.Round(sum/float64(len(levels))*1000) / 1000
	}

	avgWords := 0.0
	if total > 0 {
		avgWords = math.Round(float64(totalWords)/float64(total)*10) / 10
	}

	verbIdx := 0.0
	if total > 0 {
		verbose := float64(levelCounts[LevelVerbose] + levelCounts[LevelExcessive])
		verbIdx = math.Round(verbose/float64(total)*1000) / 1000
	}

	return ModelVerbosity{
		Model:          name,
		OverallScore:   overall,
		Grade:          grade(overall),
		Levels:         levels,
		AvgWordCount:   avgWords,
		VerbosityIndex: verbIdx,
		TotalFindings:  total,
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Verbosity Analysis (mean score: %.3f)\n\n", r.MeanScore)
	if r.BestModel != "" {
		fmt.Fprintf(&b, "  Best:  %s\n", r.BestModel)
		fmt.Fprintf(&b, "  Worst: %s\n\n", r.WorstModel)
	}
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  %s [%s] score=%.3f findings=%d avg_words=%.1f verbosity_idx=%.3f\n",
			m.Model, m.Grade, m.OverallScore, m.TotalFindings, m.AvgWordCount, m.VerbosityIndex)
		for _, l := range m.Levels {
			if l.FindingCount > 0 {
				fmt.Fprintf(&b, "    %-20s count=%d high=%d score=%.3f\n",
					l.Level, l.FindingCount, l.HighCount, l.Score)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data) + "\n"
}
