package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/contextual_precision"
)

func init() {
	Register("contextual-precision", "score response contextual precision, relevance, and specificity", runContextualPrecision)
}

func runContextualPrecision(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay contextual-precision", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay contextual-precision: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay contextual-precision: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "contextual-precision", err)
	}

	report := contextual_precision.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = contextual_precision.FormatJSON(report)
	default:
		output = contextual_precision.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "contextual-precision", err)
	}

	return ExitPass
}
