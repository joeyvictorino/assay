package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/verbosity"
)

func init() {
	Register("verbosity", "analyze response verbosity and conciseness", runVerbosity)
}

func runVerbosity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay verbosity", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay verbosity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay verbosity: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "verbosity", err)
	}

	report := verbosity.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = verbosity.FormatJSON(report)
	default:
		output = verbosity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "verbosity", err)
	}

	return ExitPass
}
