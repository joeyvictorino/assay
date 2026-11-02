package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/conciseness"
)

func init() {
	Register("conciseness", "score response conciseness across models", runConciseness)
}

func runConciseness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay conciseness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay conciseness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay conciseness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "conciseness", err)
	}

	report := conciseness.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = conciseness.FormatJSON(report)
	default:
		output = conciseness.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "conciseness", err)
	}

	return ExitPass
}
