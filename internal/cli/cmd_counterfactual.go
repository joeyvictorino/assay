package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/counterfactual"
)

func init() {
	Register("counterfactual", "score counterfactual reasoning quality, alternative generation, and causal analysis", runCounterfactual)
}

func runCounterfactual(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay counterfactual", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay counterfactual: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay counterfactual: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "counterfactual", err)
	}

	report := counterfactual.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = counterfactual.FormatJSON(report)
	default:
		output = counterfactual.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "counterfactual", err)
	}

	return ExitPass
}
