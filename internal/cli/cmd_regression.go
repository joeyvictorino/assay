package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/regression"
)

func init() {
	Register("regression", "detect score regressions between consecutive runs", runRegression)
}

func runRegression(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay regression", stderr)
	baseDir := fs.String("base", "", "path to base run directory (required)")
	curDir := fs.String("current", "", "path to current run directory (required)")
	format := fs.String("format", "text", "output format: text or json")
	threshold := fs.Float64("threshold", 0.05, "minimum score delta to count as regression (0-1)")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *baseDir == "" || *curDir == "" {
		fmt.Fprintln(stderr, "assay regression: --base and --current are required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay regression: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	baseRun, err := readRunReport(filepath.Join(*baseDir, runJSON))
	if err != nil {
		return fail(stderr, "regression", err)
	}
	curRun, err := readRunReport(filepath.Join(*curDir, runJSON))
	if err != nil {
		return fail(stderr, "regression", err)
	}

	report := regression.Detect(baseRun, curRun, *threshold)

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fail(stderr, "regression", err)
		}
		output = string(b) + "\n"
	default:
		output = regression.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "regression", err)
	}

	if report.HasRegression {
		return ExitFail
	}
	return ExitPass
}
