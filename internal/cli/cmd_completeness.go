package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/completeness"
)

func init() {
	Register("completeness", "score response completeness and topic coverage", runCompleteness)
}

func runCompleteness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay completeness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay completeness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay completeness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "completeness", err)
	}

	report := completeness.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = completeness.FormatJSON(report)
	default:
		output = completeness.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "completeness", err)
	}

	return ExitPass
}
