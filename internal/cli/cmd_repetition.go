package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/repetition"
)

func init() {
	Register("repetition", "detect repetitive and redundant language in model responses", runRepetition)
}

func runRepetition(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay repetition", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay repetition: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay repetition: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "repetition", err)
	}

	report := repetition.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = repetition.FormatJSON(report)
	default:
		output = repetition.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "repetition", err)
	}

	return ExitPass
}
