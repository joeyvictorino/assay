package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/actionability"
)

func init() {
	Register("actionability", "analyze response actionability scoring", runActionability)
}

func runActionability(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay actionability", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay actionability: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay actionability: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "actionability", err)
	}

	report := actionability.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = actionability.FormatJSON(report)
	default:
		output = actionability.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "actionability", err)
	}

	return ExitPass
}
