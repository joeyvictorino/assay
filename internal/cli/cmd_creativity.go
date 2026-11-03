package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/creativity"
)

func init() {
	Register("creativity", "score creative and original response quality", runCreativity)
}

func runCreativity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay creativity", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay creativity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay creativity: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "creativity", err)
	}

	report := creativity.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = creativity.FormatJSON(report)
	default:
		output = creativity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "creativity", err)
	}

	return ExitPass
}
