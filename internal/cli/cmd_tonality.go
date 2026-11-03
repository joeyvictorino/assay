package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/tonality"
)

func init() {
	Register("tonality", "score response tone quality, formality, empathy, and adaptability", runTonality)
}

func runTonality(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay tonality", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay tonality: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay tonality: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "tonality", err)
	}

	report := tonality.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = tonality.FormatJSON(report)
	default:
		output = tonality.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "tonality", err)
	}

	return ExitPass
}
