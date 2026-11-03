package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/fleet"
	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("fleet", "compare models across a multi-model run", runFleet)
}

func runFleet(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay fleet", stderr)
	runDir := fs.String("path", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	sortField := fs.String("sort", "score", "sort models by: score, findings, or pass-rate")
	filterModel := fs.String("model", "", "filter to a specific model")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" {
		fmt.Fprintln(stderr, "assay fleet: --path is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay fleet: unknown --format %q (text | json)\n", *format)
		return ExitError
	}
	switch *sortField {
	case "score", "findings", "pass-rate":
	default:
		fmt.Fprintf(stderr, "assay fleet: unknown --sort %q (score | findings | pass-rate)\n", *sortField)
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*runDir, runJSON))
	if err != nil {
		return fail(stderr, "fleet", err)
	}

	var findings []model.Finding
	if p := filepath.Join(*runDir, findingsJSON); fileExists(p) {
		if findings, err = readFindings(p, nil); err != nil {
			return fail(stderr, "fleet", err)
		}
	}

	report := fleet.AnalyzeFleet(rr, findings)

	if *filterModel != "" {
		report = fleet.FilterModel(report, *filterModel)
	}
	if *sortField != "score" {
		report = fleet.SortBy(report, *sortField)
	}

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fail(stderr, "fleet", err)
		}
		output = string(b) + "\n"
	default:
		output = fleet.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "fleet", err)
	}
	return ExitPass
}
