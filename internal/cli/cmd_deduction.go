package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/deduction"
)

func init() {
	Register("deduction", "score deductive reasoning quality, logical chaining, and conclusion validity", runDeduction)
}

func runDeduction(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay deduction", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay deduction: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay deduction: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "deduction", err)
	}

	report := deduction.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = deduction.FormatJSON(report)
	default:
		output = deduction.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "deduction", err)
	}

	return ExitPass
}
