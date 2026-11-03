package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/accessibility"
)

func init() {
	Register("accessibility", "score plain language, structure clarity, and audience awareness", runAccessibility)
}

func runAccessibility(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay accessibility", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay accessibility: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay accessibility: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "accessibility", err)
	}

	report := accessibility.Analyze(data)
	var output string
	switch *format {
	case "json":
		output = accessibility.FormatJSON(report)
	default:
		output = accessibility.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "accessibility", err)
	}
	return ExitPass
}
