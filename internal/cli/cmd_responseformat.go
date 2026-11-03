package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/responseformat"
)

func init() {
	Register("response-format", "score response format compliance across models", runResponseFormat)
}

func runResponseFormat(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay response-format", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay response-format: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay response-format: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "response-format", err)
	}

	report := responseformat.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = responseformat.FormatJSON(report)
	default:
		output = responseformat.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "response-format", err)
	}

	return ExitPass
}
