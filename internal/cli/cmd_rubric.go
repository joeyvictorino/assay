package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/rubric"
)

func init() {
	Register("scoring-rubric", "weighted rubric evaluation across detection, severity, FP rate, coverage, explanation quality, and consistency", runRubric)
}

func runRubric(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay scoring-rubric", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay scoring-rubric: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "scoring-rubric", err)
	}

	r := rubric.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = rubric.FormatJSON(r)
	default:
		output = rubric.FormatText(r)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "scoring-rubric", err)
	}

	return ExitPass
}
