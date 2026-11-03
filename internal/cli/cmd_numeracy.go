package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/numeracy"
)

func init() {
	Register("numeracy", "score numerical accuracy, quantitative precision, and unit consistency", runNumeracy)
}

func runNumeracy(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay numeracy", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay numeracy: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay numeracy: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "numeracy", err)
	}

	report := numeracy.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = numeracy.FormatJSON(report)
	default:
		output = numeracy.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "numeracy", err)
	}

	return ExitPass
}
