package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/hedging"
)

func init() {
	Register("hedging", "score hedging language, uncertainty, qualifiers, and disclaimers", runHedging)
}

func runHedging(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay hedging", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay hedging: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay hedging: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "hedging", err)
	}

	report := hedging.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = hedging.FormatJSON(report)
	default:
		output = hedging.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "hedging", err)
	}

	return ExitPass
}
