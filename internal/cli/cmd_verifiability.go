package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/verifiability"
)

func init() {
	Register("verifiability", "score response verifiability, testability, and reproducibility", runVerifiability)
}

func runVerifiability(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay verifiability", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay verifiability: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay verifiability: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "verifiability", err)
	}

	report := verifiability.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = verifiability.FormatJSON(report)
	default:
		output = verifiability.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "verifiability", err)
	}

	return ExitPass
}
