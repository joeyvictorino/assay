package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/compositionality"
)

func init() {
	Register("compositionality", "score compositional reasoning quality, concept integration, and synthesis depth", runCompositionality)
}

func runCompositionality(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay compositionality", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay compositionality: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay compositionality: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "compositionality", err)
	}

	report := compositionality.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = compositionality.FormatJSON(report)
	default:
		output = compositionality.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "compositionality", err)
	}

	return ExitPass
}
