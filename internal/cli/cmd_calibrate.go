package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/calibrate"
)

func init() {
	Register("calibrate", "analyze checker self-score accuracy vs peer review", runCalibrate)
}

func runCalibrate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay calibrate", stderr)
	runDir := fs.String("path", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	checker := fs.String("checker", "", "filter to a single checker/model name")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" {
		fmt.Fprintln(stderr, "assay calibrate: --path is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay calibrate: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*runDir, runJSON))
	if err != nil {
		return fail(stderr, "calibrate", err)
	}

	report := calibrate.Analyze(rr)

	if *checker != "" {
		var filtered []calibrate.CheckerCalibration
		for _, c := range report.Checkers {
			if c.Checker == *checker {
				filtered = append(filtered, c)
			}
		}
		report.Checkers = filtered
		if len(filtered) > 0 {
			report.Overall = calibrate.OverallCalibration{
				MeanBias:     filtered[0].Bias,
				MeanAbsError: filtered[0].AbsError,
				BestChecker:  filtered[0].Checker,
				WorstChecker: filtered[0].Checker,
			}
		}
	}

	switch *format {
	case "json":
		if err := writeJSON("", report, stdout); err != nil {
			return fail(stderr, "calibrate", err)
		}
	default:
		text := calibrate.FormatText(report)
		if _, err := io.WriteString(stdout, text); err != nil {
			return fail(stderr, "calibrate", err)
		}
	}
	return ExitPass
}
