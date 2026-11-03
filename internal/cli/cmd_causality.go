package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/causality"
)

func init() {
	Register("causality", "score causal reasoning quality, mechanism clarity, and counterfactual use", runCausality)
}

func runCausality(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay causality", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay causality: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay causality: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "causality", err)
	}

	report := causality.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = causality.FormatJSON(report)
	default:
		output = causality.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "causality", err)
	}

	return ExitPass
}
