package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/tact"
)

func init() {
	Register("tact", "score response tactfulness, diplomacy, and sensitivity", runTact)
}

func runTact(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay tact", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay tact: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay tact: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "tact", err)
	}

	report := tact.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = tact.FormatJSON(report)
	default:
		output = tact.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "tact", err)
	}

	return ExitPass
}
