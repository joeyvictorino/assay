package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/overrefusal"
)

func init() {
	Register("overrefusal", "detect model over-refusal patterns in outputs", runOverrefusal)
}

func runOverrefusal(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay overrefusal", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay overrefusal: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay overrefusal: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "overrefusal", err)
	}

	report := overrefusal.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = overrefusal.FormatJSON(report)
	default:
		output = overrefusal.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "overrefusal", err)
	}

	return ExitPass
}
