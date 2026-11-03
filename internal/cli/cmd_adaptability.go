package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/adaptability"
)

func init() {
	Register("adaptability", "score response adaptability across tone, format, and audience", runAdaptability)
}

func runAdaptability(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay adaptability", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay adaptability: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay adaptability: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "adaptability", err)
	}

	report := adaptability.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = adaptability.FormatJSON(report)
	default:
		output = adaptability.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "adaptability", err)
	}

	return ExitPass
}
