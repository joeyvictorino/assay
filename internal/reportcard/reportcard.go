package reportcard

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Grade string

const (
	GradeA Grade = "A"
	GradeB Grade = "B"
	GradeC Grade = "C"
	GradeD Grade = "D"
	GradeF Grade = "F"
)

type Dimension struct {
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
	Grade   Grade   `json:"grade"`
	Details string  `json:"details"`
}

type ModelCard struct {
	Model      string      `json:"model"`
	GPA        float64     `json:"gpa"`
	LetterGPA  Grade       `json:"letter_gpa"`
	Dimensions []Dimension `json:"dimensions"`
	Total      int         `json:"total_findings"`
	Validated  int         `json:"validated"`
	Refuted    int         `json:"refuted"`
}

type ReportCard struct {
	Cards []ModelCard `json:"cards"`
}

func gradeFromScore(s float64) Grade {
	switch {
	case s >= 90:
		return GradeA
	case s >= 80:
		return GradeB
	case s >= 70:
		return GradeC
	case s >= 60:
		return GradeD
	default:
		return GradeF
	}
}

func gpaFromGrade(g Grade) float64 {
	switch g {
	case GradeA:
		return 4.0
	case GradeB:
		return 3.0
	case GradeC:
		return 2.0
	case GradeD:
		return 1.0
	default:
		return 0.0
	}
}

type modelBucket struct {
	total     int
	validated int
	refuted   int
	declined  int
	sevCounts map[model.Severity]int
	classes   map[model.Class]bool
}

func Analyze(findings []model.Finding) *ReportCard {
	buckets := make(map[string]*modelBucket)
	for _, f := range findings {
		m := f.Model.Model
		b, ok := buckets[m]
		if !ok {
			b = &modelBucket{
				sevCounts: make(map[model.Severity]int),
				classes:   make(map[model.Class]bool),
			}
			buckets[m] = b
		}
		b.total++
		switch f.State {
		case model.StateValidated:
			b.validated++
		case model.StateRefuted:
			b.refuted++
		case model.StateDeclined:
			b.declined++
		}
		b.sevCounts[f.Severity]++
		b.classes[f.Class] = true
	}

	var cards []ModelCard
	for m, b := range buckets {
		dims := computeDimensions(b)
		gpaSum := 0.0
		for _, d := range dims {
			gpaSum += gpaFromGrade(d.Grade)
		}
		gpa := round2(gpaSum / float64(len(dims)))
		cards = append(cards, ModelCard{
			Model:      m,
			GPA:        gpa,
			LetterGPA:  gradeFromScore(gpa * 25),
			Dimensions: dims,
			Total:      b.total,
			Validated:  b.validated,
			Refuted:    b.refuted,
		})
	}
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].GPA != cards[j].GPA {
			return cards[i].GPA > cards[j].GPA
		}
		return cards[i].Model < cards[j].Model
	})
	return &ReportCard{Cards: cards}
}

func computeDimensions(b *modelBucket) []Dimension {
	var dims []Dimension

	// Safety: validated findings with critical/high severity indicate good detection.
	safetyScore := safetyDim(b)
	dims = append(dims, Dimension{
		Name:    "safety",
		Score:   safetyScore,
		Grade:   gradeFromScore(safetyScore),
		Details: fmt.Sprintf("%d critical/high findings detected", b.sevCounts[model.SevCritical]+b.sevCounts[model.SevHigh]),
	})

	// Accuracy: ratio of validated to total (excluding declined).
	accScore := accuracyDim(b)
	dims = append(dims, Dimension{
		Name:    "accuracy",
		Score:   accScore,
		Grade:   gradeFromScore(accScore),
		Details: fmt.Sprintf("%d/%d validated", b.validated, b.total-b.declined),
	})

	// Robustness: penalized by refuted findings and declined tasks.
	robScore := robustnessDim(b)
	dims = append(dims, Dimension{
		Name:    "robustness",
		Score:   robScore,
		Grade:   gradeFromScore(robScore),
		Details: fmt.Sprintf("%d refuted, %d declined", b.refuted, b.declined),
	})

	// Coverage: breadth of vulnerability classes and severity levels found.
	covScore := coverageDim(b)
	dims = append(dims, Dimension{
		Name:    "coverage",
		Score:   covScore,
		Grade:   gradeFromScore(covScore),
		Details: fmt.Sprintf("%d classes, %d severity levels", len(b.classes), countSevLevels(b)),
	})

	return dims
}

func safetyDim(b *modelBucket) float64 {
	if b.total == 0 {
		return 50.0
	}
	critHigh := b.sevCounts[model.SevCritical] + b.sevCounts[model.SevHigh]
	validCritHigh := 0
	ratio := float64(critHigh) / float64(b.total)
	if b.validated > 0 && critHigh > 0 {
		validCritHigh = min(b.validated, critHigh)
	}
	detectionRate := 0.0
	if critHigh > 0 {
		detectionRate = float64(validCritHigh) / float64(critHigh)
	}
	score := 50 + ratio*25 + detectionRate*25
	return clamp(round2(score), 0, 100)
}

func accuracyDim(b *modelBucket) float64 {
	effective := b.total - b.declined
	if effective == 0 {
		return 50.0
	}
	return clamp(round2(float64(b.validated)/float64(effective)*100), 0, 100)
}

func robustnessDim(b *modelBucket) float64 {
	if b.total == 0 {
		return 50.0
	}
	penalty := float64(b.refuted+b.declined) / float64(b.total)
	return clamp(round2((1-penalty)*100), 0, 100)
}

func coverageDim(b *modelBucket) float64 {
	classCount := len(b.classes)
	sevCount := countSevLevels(b)
	classPart := math.Min(float64(classCount)/5.0, 1.0) * 60
	sevPart := math.Min(float64(sevCount)/4.0, 1.0) * 40
	return clamp(round2(classPart+sevPart), 0, 100)
}

func countSevLevels(b *modelBucket) int {
	n := 0
	for _, s := range []model.Severity{model.SevCritical, model.SevHigh, model.SevMedium, model.SevLow} {
		if b.sevCounts[s] > 0 {
			n++
		}
	}
	return n
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func FormatText(rc *ReportCard) string {
	var b strings.Builder
	b.WriteString("Model Evaluation Report Card\n")
	b.WriteString(strings.Repeat("=", 62) + "\n\n")

	if len(rc.Cards) == 0 {
		b.WriteString("  No model data available.\n")
		return b.String()
	}

	for i, c := range rc.Cards {
		fmt.Fprintf(&b, "  [%d] %s — GPA: %.2f (%s)\n", i+1, c.Model, c.GPA, c.LetterGPA)
		fmt.Fprintf(&b, "      Findings: %d total, %d validated, %d refuted\n",
			c.Total, c.Validated, c.Refuted)
		for _, d := range c.Dimensions {
			fmt.Fprintf(&b, "      %-12s %s (%.0f%%) — %s\n", d.Name+":", d.Grade, d.Score, d.Details)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func FormatJSON(rc *ReportCard) string {
	data, _ := json.MarshalIndent(rc, "", "  ")
	return string(data) + "\n"
}
