package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/verbosityscore"
)

func init() {
	Register("verbosity-score", "score response verbosity across models", runVerbosityScore)
}

func runVerbosityScore(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay verbosity-score", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay verbosity-score: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay verbosity-score: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "verbosity-score", err)
	}

	report := verbosityscore.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = verbosityscore.FormatJSON(report)
	default:
		output = verbosityscore.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "verbosity-score", err)
	}

	return ExitPass
}
