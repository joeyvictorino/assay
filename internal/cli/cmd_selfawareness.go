package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/selfawareness"
)

func init() {
	Register("self-awareness", "score model self-awareness about uncertainty and limitations", runSelfAwareness)
}

func runSelfAwareness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay self-awareness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay self-awareness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay self-awareness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "self-awareness", err)
	}

	report := selfawareness.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = selfawareness.FormatJSON(report)
	default:
		output = selfawareness.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "self-awareness", err)
	}

	return ExitPass
}
