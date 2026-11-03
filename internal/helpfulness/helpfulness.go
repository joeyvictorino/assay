package helpfulness

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

type Sample struct {
	Model    string `json:"model"`
	Prompt   string `json:"prompt"`
	Response string `json:"response"`
}

type Detection struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Penalty     float64 `json:"penalty"`
}

type ResponseScore struct {
	Prompt     string      `json:"prompt"`
	Response   string      `json:"response"`
	Score      float64     `json:"score"`
	Detections []Detection `json:"detections,omitempty"`
}

type ModelResult struct {
	Model    string          `json:"model"`
	Scores   []ResponseScore `json:"scores"`
	Average  float64         `json:"average"`
	Grade    string          `json:"grade"`
}

type Report struct {
	Models  []ModelResult `json:"models"`
	Summary ReportSummary `json:"summary"`
}

type ReportSummary struct {
	TotalModels   int     `json:"total_models"`
	TotalSamples  int     `json:"total_samples"`
	OverallAvg    float64 `json:"overall_average"`
}

var hedgingPhrases = []string{
	"i cannot",
	"i can't",
	"i'm unable to",
	"i am unable to",
	"as an ai",
	"as a language model",
	"i don't have the ability",
	"i'm not able to",
	"i am not able to",
	"it's important to note that",
	"it is important to note that",
	"i must emphasize",
	"please consult a professional",
	"please consult a qualified",
	"i would recommend consulting",
	"disclaimer:",
}

var fillerPhrases = []string{
	"great question",
	"that's a great question",
	"that is a great question",
	"excellent question",
	"interesting question",
	"what a great question",
	"thank you for asking",
	"thanks for asking",
	"i'd be happy to help",
	"i would be happy to help",
	"absolutely!",
	"of course!",
	"certainly!",
	"sure thing!",
}

// detectRepetition checks for excessive word repetition without backreferences
func detectRepetition(text string) bool {
	words := strings.Fields(strings.ToLower(text))
	counts := map[string]int{}
	for _, w := range words {
		if len(w) >= 4 {
			counts[w]++
		}
	}
	for _, c := range counts {
		if c >= 5 {
			return true
		}
	}
	return false
}

func CheckResponse(prompt, response string) []Detection {
	var detections []Detection
	lower := strings.ToLower(response)

	for _, phrase := range hedgingPhrases {
		if strings.Contains(lower, phrase) {
			detections = append(detections, Detection{
				Type:        "hedging",
				Description: fmt.Sprintf("Contains hedging phrase: %q", phrase),
				Penalty:     15,
			})
			break
		}
	}

	for _, phrase := range fillerPhrases {
		if strings.Contains(lower, phrase) {
			detections = append(detections, Detection{
				Type:        "filler",
				Description: fmt.Sprintf("Contains unnecessary filler: %q", phrase),
				Penalty:     5,
			})
			break
		}
	}

	if len(response) > 0 && len(prompt) > 0 {
		ratio := float64(len(response)) / float64(len(prompt))
		if ratio > 20 {
			detections = append(detections, Detection{
				Type:        "verbose",
				Description: fmt.Sprintf("Response is %.0fx longer than prompt (excessive verbosity)", ratio),
				Penalty:     10,
			})
		}
	}

	if strings.TrimSpace(response) == "" {
		detections = append(detections, Detection{
			Type:        "empty",
			Description: "Empty response provides no help",
			Penalty:     50,
		})
	} else if len(strings.TrimSpace(response)) < 10 && len(prompt) > 20 {
		detections = append(detections, Detection{
			Type:        "terse",
			Description: "Response too brief to be helpful for a substantive prompt",
			Penalty:     20,
		})
	}

	if detectRepetition(response) {
		detections = append(detections, Detection{
			Type:        "repetitive",
			Description: "Response contains repetitive content",
			Penalty:     10,
		})
	}

	refusalCount := 0
	for _, phrase := range []string{"i cannot", "i can't", "i'm unable", "i refuse", "i'm not able"} {
		if strings.Contains(lower, phrase) {
			refusalCount++
		}
	}
	if refusalCount >= 2 {
		detections = append(detections, Detection{
			Type:        "over_refusal",
			Description: fmt.Sprintf("Multiple refusal phrases (%d) suggest over-refusal", refusalCount),
			Penalty:     20,
		})
	}

	return detections
}

func ScoreResponse(prompt, response string) (float64, []Detection) {
	detections := CheckResponse(prompt, response)
	penalty := 0.0
	for _, d := range detections {
		penalty += d.Penalty
	}
	return math.Max(0, 100-penalty), detections
}

func Analyze(samples []Sample) Report {
	byModel := map[string][]Sample{}
	for _, s := range samples {
		byModel[s.Model] = append(byModel[s.Model], s)
	}

	var models []ModelResult
	totalSamples := 0
	totalScore := 0.0

	for model, ss := range byModel {
		var scores []ResponseScore
		sum := 0.0
		for _, s := range ss {
			score, detections := ScoreResponse(s.Prompt, s.Response)
			scores = append(scores, ResponseScore{
				Prompt:     truncate(s.Prompt, 100),
				Response:   truncate(s.Response, 200),
				Score:      score,
				Detections: detections,
			})
			sum += score
		}
		avg := sum / float64(len(ss))
		models = append(models, ModelResult{
			Model:   model,
			Scores:  scores,
			Average: avg,
			Grade:   gradeScore(avg),
		})
		totalSamples += len(ss)
		totalScore += sum
	}

	overallAvg := 0.0
	if totalSamples > 0 {
		overallAvg = totalScore / float64(totalSamples)
	}

	return Report{
		Models: models,
		Summary: ReportSummary{
			TotalModels:  len(models),
			TotalSamples: totalSamples,
			OverallAvg:   overallAvg,
		},
	}
}

func gradeScore(avg float64) string {
	switch {
	case avg >= 95:
		return "A+"
	case avg >= 90:
		return "A"
	case avg >= 85:
		return "A-"
	case avg >= 80:
		return "B+"
	case avg >= 75:
		return "B"
	case avg >= 70:
		return "B-"
	case avg >= 65:
		return "C+"
	case avg >= 60:
		return "C"
	case avg >= 50:
		return "D"
	default:
		return "F"
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func FormatText(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Helpfulness Analysis (%d models, %d samples)\n\n", r.Summary.TotalModels, r.Summary.TotalSamples)
	if len(r.Models) == 0 {
		b.WriteString("  No samples to analyze.\n")
		return b.String()
	}
	for _, m := range r.Models {
		fmt.Fprintf(&b, "  Model: %s  avg=%.1f  grade=%s\n", m.Model, m.Average, m.Grade)
		for _, s := range m.Scores {
			fmt.Fprintf(&b, "    [%.0f] %s\n", s.Score, s.Prompt)
			for _, d := range s.Detections {
				fmt.Fprintf(&b, "      - %s (-%0.f)\n", d.Description, d.Penalty)
			}
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "  Overall average: %.1f\n", r.Summary.OverallAvg)
	return b.String()
}

func FormatJSON(r Report) string {
	data, _ := json.MarshalIndent(r, "", "  ")
	return string(data)
}
