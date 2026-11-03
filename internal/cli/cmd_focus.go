package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/focus"
)

func init() {
	Register("focus", "score response focus, topic adherence, and digression detection", runFocus)
}

func runFocus(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay focus", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay focus: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay focus: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "focus", err)
	}

	report := focus.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = focus.FormatJSON(report)
	default:
		output = focus.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "focus", err)
	}

	return ExitPass
}
