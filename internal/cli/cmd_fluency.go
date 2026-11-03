package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/fluency"
)

func init() {
	Register("fluency", "score language fluency, naturalness, and readability", runFluency)
}

func runFluency(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay fluency", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay fluency: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay fluency: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "fluency", err)
	}

	report := fluency.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = fluency.FormatJSON(report)
	default:
		output = fluency.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "fluency", err)
	}

	return ExitPass
}
