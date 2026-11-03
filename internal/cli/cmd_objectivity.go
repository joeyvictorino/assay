package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/objectivity"
)

func init() {
	Register("objectivity", "score response objectivity, impartial tone, and evidence-based reasoning", runObjectivity)
}

func runObjectivity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay objectivity", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay objectivity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay objectivity: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "objectivity", err)
	}

	report := objectivity.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = objectivity.FormatJSON(report)
	default:
		output = objectivity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "objectivity", err)
	}

	return ExitPass
}
