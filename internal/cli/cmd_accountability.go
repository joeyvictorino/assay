package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/accountability"
)

func init() {
	Register("accountability", "score response accountability and deflection detection", runAccountability)
}

func runAccountability(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay accountability", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay accountability: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay accountability: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "accountability", err)
	}

	report := accountability.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = accountability.FormatJSON(report)
	default:
		output = accountability.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "accountability", err)
	}

	return ExitPass
}
