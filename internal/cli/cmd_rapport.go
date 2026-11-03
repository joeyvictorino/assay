package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/rapport"
)

func init() {
	Register("rapport", "score response rapport-building, warmth, and personalization", runRapport)
}

func runRapport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay rapport", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay rapport: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay rapport: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "rapport", err)
	}

	report := rapport.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = rapport.FormatJSON(report)
	default:
		output = rapport.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "rapport", err)
	}

	return ExitPass
}
