package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/uncertainty"
)

func init() {
	Register("uncertainty", "score uncertainty calibration and confidence in model responses", runUncertainty)
}

func runUncertainty(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay uncertainty", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay uncertainty: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay uncertainty: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "uncertainty", err)
	}

	report := uncertainty.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = uncertainty.FormatJSON(report)
	default:
		output = uncertainty.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "uncertainty", err)
	}

	return ExitPass
}
