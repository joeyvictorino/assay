package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/directness"
)

func init() {
	Register("directness", "score response directness, prompt alignment, and answer efficiency", runDirectness)
}

func runDirectness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay directness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay directness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay directness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "directness", err)
	}

	report := directness.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = directness.FormatJSON(report)
	default:
		output = directness.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "directness", err)
	}

	return ExitPass
}
