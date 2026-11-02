package checkermatrix

import (
	"fmt"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

type CellResult struct {
	Ran   bool    `json:"ran"`
	Count int     `json:"count"`
	Score float64 `json:"score"`
}

type MatrixResult struct {
	Checkers []string              `json:"checkers"`
	Models   []string              `json:"models"`
	Matrix   map[string]CellResult `json:"matrix"`
}

func cellKey(checker, modelID string) string {
	return checker + ":" + modelID
}

func Build(findings []model.Finding) MatrixResult {
	checkerSet := map[string]bool{}
	modelSet := map[string]bool{}
	cells := map[string]*CellResult{}

	for _, f := range findings {
		checker := f.Lab
		if checker == "" {
			checker = "(unknown)"
		}
		modelID := f.Model.Model
		if modelID == "" {
			modelID = "(unknown)"
		}

		checkerSet[checker] = true
		modelSet[modelID] = true

		key := cellKey(checker, modelID)
		c, ok := cells[key]
		if !ok {
			c = &CellResult{Ran: true}
			cells[key] = c
		}
		c.Count++
		c.Score += float64(f.Severity.Rank())
	}

	for key, c := range cells {
		if c.Count > 0 {
			c.Score = c.Score / float64(c.Count)
		}
		cells[key] = c
	}

	checkers := make([]string, 0, len(checkerSet))
	for c := range checkerSet {
		checkers = append(checkers, c)
	}
	sort.Strings(checkers)

	models := make([]string, 0, len(modelSet))
	for m := range modelSet {
		models = append(models, m)
	}
	sort.Strings(models)

	flat := make(map[string]CellResult, len(cells))
	for k, v := range cells {
		flat[k] = *v
	}

	return MatrixResult{
		Checkers: checkers,
		Models:   models,
		Matrix:   flat,
	}
}

func FormatText(m MatrixResult) string {
	if len(m.Checkers) == 0 || len(m.Models) == 0 {
		return "No checker/model data found.\n"
	}

	var b strings.Builder

	maxChecker := 7
	for _, c := range m.Checkers {
		if len(c) > maxChecker {
			maxChecker = len(c)
		}
	}

	colWidth := 12
	for _, mod := range m.Models {
		if len(mod)+2 > colWidth {
			colWidth = len(mod) + 2
		}
	}

	fmt.Fprintf(&b, "%-*s", maxChecker+2, "Checker")
	for _, mod := range m.Models {
		fmt.Fprintf(&b, "%-*s", colWidth, mod)
	}
	b.WriteString("\n")
	b.WriteString(strings.Repeat("-", maxChecker+2+colWidth*len(m.Models)))
	b.WriteString("\n")

	for _, checker := range m.Checkers {
		fmt.Fprintf(&b, "%-*s", maxChecker+2, checker)
		for _, mod := range m.Models {
			key := cellKey(checker, mod)
			cell, ok := m.Matrix[key]
			if !ok || !cell.Ran {
				fmt.Fprintf(&b, "%-*s", colWidth, "-")
			} else {
				fmt.Fprintf(&b, "%-*s", colWidth, fmt.Sprintf("%d (%.1f)", cell.Count, cell.Score))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	fmt.Fprintf(&b, "Checkers: %d  Models: %d\n", len(m.Checkers), len(m.Models))
	return b.String()
}
