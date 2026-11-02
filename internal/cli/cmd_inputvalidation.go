package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/inputvalidation"
)

func init() {
	Register("input-validation", "test model input handling for edge cases and adversarial inputs", runInputValidation)
}

func runInputValidation(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay input-validation", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay input-validation: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay input-validation: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "input-validation", err)
	}

	report := inputvalidation.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = inputvalidation.FormatJSON(report)
	default:
		output = inputvalidation.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "input-validation", err)
	}

	return ExitPass
}
