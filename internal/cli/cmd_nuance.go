package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/nuance"
)

func init() {
	Register("nuance", "score response nuance, edge-case handling, and balanced perspective", runNuance)
}

func runNuance(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay nuance", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay nuance: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay nuance: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "nuance", err)
	}

	report := nuance.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = nuance.FormatJSON(report)
	default:
		output = nuance.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "nuance", err)
	}

	return ExitPass
}
