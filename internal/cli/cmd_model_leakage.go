package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/modelleakage"
)

func init() {
	Register("model-leakage", "detect model information leakage in outputs", runModelLeakage)
}

func runModelLeakage(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-leakage", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-leakage: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay model-leakage: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "model-leakage", err)
	}

	report := modelleakage.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = modelleakage.FormatJSON(report)
	default:
		output = modelleakage.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-leakage", err)
	}

	return ExitPass
}
