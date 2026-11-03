package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/modelfairness"
)

func init() {
	Register("model-fairness", "evaluate fairness of finding distributions across models", runModelFairness)
}

func runModelFairness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-fairness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-fairness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay model-fairness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "model-fairness", err)
	}

	report := modelfairness.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = modelfairness.FormatJSON(report)
	default:
		output = modelfairness.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-fairness", err)
	}

	return ExitPass
}
