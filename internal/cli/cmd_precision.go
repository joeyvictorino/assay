package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/precision"
)

func init() {
	Register("precision", "score response factual precision, citation quality, and technical accuracy", runPrecision)
}

func runPrecision(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay precision", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay precision: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay precision: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "precision", err)
	}

	report := precision.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = precision.FormatJSON(report)
	default:
		output = precision.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "precision", err)
	}

	return ExitPass
}
