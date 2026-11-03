package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/falsepos"
)

func init() {
	Register("false-positive", "estimate false-positive rates from validation results", runFalsePos)
}

func runFalsePos(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay false-positive", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay false-positive: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "false-positive", err)
	}

	report := falsepos.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = falsepos.FormatJSON(report)
	default:
		output = falsepos.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "false-positive", err)
	}

	return ExitPass
}
