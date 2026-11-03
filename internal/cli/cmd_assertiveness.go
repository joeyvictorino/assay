package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/assertiveness"
)

func init() {
	Register("assertiveness", "score response assertiveness, confidence, and decisiveness", runAssertiveness)
}

func runAssertiveness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay assertiveness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay assertiveness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay assertiveness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "assertiveness", err)
	}

	report := assertiveness.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = assertiveness.FormatJSON(report)
	default:
		output = assertiveness.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "assertiveness", err)
	}

	return ExitPass
}
