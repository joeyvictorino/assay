package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/formality"
)

func init() {
	Register("formality", "score response formality level, register appropriateness, and professional tone", runFormality)
}

func runFormality(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay formality", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay formality: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay formality: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "formality", err)
	}

	report := formality.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = formality.FormatJSON(report)
	default:
		output = formality.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "formality", err)
	}

	return ExitPass
}
