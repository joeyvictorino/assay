package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/fairness"
)

func init() {
	Register("fairness", "evaluate model fairness across demographic groups", runFairness)
}

func runFairness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay fairness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay fairness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay fairness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "fairness", err)
	}

	report := fairness.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = fairness.FormatJSON(report)
	default:
		output = fairness.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "fairness", err)
	}

	return ExitPass
}
