package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/sensitivity"
)

func init() {
	Register("sensitivity", "analyze prompt sensitivity per checker", runSensitivity)
}

func runSensitivity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay sensitivity", stderr)
	runDir := fs.String("path", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	checker := fs.String("checker", "", "filter to a single checker name")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" {
		fmt.Fprintln(stderr, "assay sensitivity: --path is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay sensitivity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*runDir, runJSON))
	if err != nil {
		return fail(stderr, "sensitivity", err)
	}

	report := sensitivity.Analyze(rr)

	if *checker != "" {
		var filtered []sensitivity.CheckerSensitivity
		for _, c := range report.Checkers {
			if c.Checker == *checker {
				filtered = append(filtered, c)
			}
		}
		report.Checkers = filtered
		if len(filtered) > 0 {
			report.MostStable = filtered[0].Checker
			report.MostSensitive = filtered[0].Checker
			report.OverallCV = filtered[0].CV
		}
	}

	switch *format {
	case "json":
		if err := writeJSON("", report, stdout); err != nil {
			return fail(stderr, "sensitivity", err)
		}
	default:
		text := sensitivity.FormatText(report)
		if _, err := io.WriteString(stdout, text); err != nil {
			return fail(stderr, "sensitivity", err)
		}
	}
	return ExitPass
}
