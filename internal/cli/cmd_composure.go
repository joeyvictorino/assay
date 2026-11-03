package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/composure"
)

func init() {
	Register("composure", "score response composure, hedging ratio, and confidence signals", runComposure)
}

func runComposure(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay composure", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay composure: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay composure: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "composure", err)
	}

	report := composure.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = composure.FormatJSON(report)
	default:
		output = composure.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "composure", err)
	}

	return ExitPass
}
