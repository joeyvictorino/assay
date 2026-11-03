package regression

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type Regression struct {
	Model    string  `json:"model"`
	Checker  string  `json:"checker"`
	OldScore float64 `json:"old_score"`
	NewScore float64 `json:"new_score"`
	Delta    float64 `json:"delta"`
	Severity string  `json:"severity"`
}

type RegressionReport struct {
	BaseRun       string       `json:"base_run"`
	CurrentRun    string       `json:"current_run"`
	Regressions   []Regression `json:"regressions"`
	Improvements  []Regression `json:"improvements"`
	Stable        int          `json:"stable_count"`
	HasRegression bool         `json:"has_regression"`
}

type reflKey struct {
	Model   string
	Checker string
}

func scoreSeverity(delta float64) string {
	absDelta := math.Abs(delta)
	if absDelta > 0.3 {
		return "critical"
	}
	if absDelta > 0.1 {
		return "warning"
	}
	return "info"
}

func Detect(base, current model.RunReport, threshold float64) RegressionReport {
	baseScores := map[reflKey]float64{}
	for _, r := range base.Reflection {
		k := reflKey{Model: r.Model, Checker: r.Lab}
		baseScores[k] = float64(r.Final) / 10.0
	}

	curScores := map[reflKey]float64{}
	for _, r := range current.Reflection {
		k := reflKey{Model: r.Model, Checker: r.Lab}
		curScores[k] = float64(r.Final) / 10.0
	}

	var regressions, improvements []Regression
	stable := 0

	allKeys := map[reflKey]bool{}
	for k := range baseScores {
		allKeys[k] = true
	}
	for k := range curScores {
		allKeys[k] = true
	}

	for k := range allKeys {
		oldScore, hasOld := baseScores[k]
		newScore, hasNew := curScores[k]
		if !hasOld || !hasNew {
			continue
		}
		delta := newScore - oldScore
		absDelta := math.Abs(delta)
		if absDelta < threshold {
			stable++
			continue
		}
		entry := Regression{
			Model:    k.Model,
			Checker:  k.Checker,
			OldScore: oldScore,
			NewScore: newScore,
			Delta:    delta,
			Severity: scoreSeverity(delta),
		}
		if delta < 0 {
			regressions = append(regressions, entry)
		} else {
			improvements = append(improvements, entry)
		}
	}

	sort.Slice(regressions, func(i, j int) bool {
		return regressions[i].Delta < regressions[j].Delta
	})
	sort.Slice(improvements, func(i, j int) bool {
		return improvements[i].Delta > improvements[j].Delta
	})

	hasCritical := false
	for _, r := range regressions {
		if r.Severity == "critical" {
			hasCritical = true
			break
		}
	}

	return RegressionReport{
		BaseRun:       base.RunID,
		CurrentRun:    current.RunID,
		Regressions:   regressions,
		Improvements:  improvements,
		Stable:        stable,
		HasRegression: hasCritical,
	}
}

func FormatText(r RegressionReport) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Regression Report: %s → %s\n", r.BaseRun, r.CurrentRun)
	fmt.Fprintf(&b, "  Regressions: %d  Improvements: %d  Stable: %d\n\n",
		len(r.Regressions), len(r.Improvements), r.Stable)

	if len(r.Regressions) > 0 {
		fmt.Fprintf(&b, "Regressions (%d):\n", len(r.Regressions))
		for _, reg := range r.Regressions {
			fmt.Fprintf(&b, "  ▼ %-20s %-20s %.2f → %.2f (%+.2f) [%s]\n",
				reg.Model, reg.Checker, reg.OldScore, reg.NewScore, reg.Delta, reg.Severity)
		}
		b.WriteString("\n")
	}

	if len(r.Improvements) > 0 {
		fmt.Fprintf(&b, "Improvements (%d):\n", len(r.Improvements))
		for _, imp := range r.Improvements {
			fmt.Fprintf(&b, "  ▲ %-20s %-20s %.2f → %.2f (%+.2f)\n",
				imp.Model, imp.Checker, imp.OldScore, imp.NewScore, imp.Delta)
		}
		b.WriteString("\n")
	}

	if len(r.Regressions) == 0 && len(r.Improvements) == 0 {
		b.WriteString("No significant score changes detected.\n")
	}

	if r.HasRegression {
		b.WriteString("⚠ Critical regressions detected.\n")
	}

	return b.String()
}
