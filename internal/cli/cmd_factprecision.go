package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/factprecision"
)

func init() {
	Register("fact-precision", "score factual precision and claim verifiability", runFactPrecision)
}

func runFactPrecision(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay fact-precision", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay fact-precision: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay fact-precision: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "fact-precision", err)
	}

	report := factprecision.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = factprecision.FormatJSON(report)
	default:
		output = factprecision.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "fact-precision", err)
	}

	return ExitPass
}
