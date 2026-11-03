package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/memorization"
)

func init() {
	Register("memorization", "detect memorized training data in model outputs", runMemorization)
}

func runMemorization(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay memorization", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay memorization: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay memorization: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "memorization", err)
	}

	report := memorization.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = memorization.FormatJSON(report)
	default:
		output = memorization.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "memorization", err)
	}

	return ExitPass
}
