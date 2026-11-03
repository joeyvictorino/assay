package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/costcompare"
)

func init() {
	Register("cost-compare", "compare cost efficiency across models", runCostCompare)
}

func runCostCompare(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay cost-compare", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay cost-compare: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "cost-compare", err)
	}

	r := costcompare.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = costcompare.FormatJSON(r)
	default:
		output = costcompare.FormatText(r)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "cost-compare", err)
	}

	return ExitPass
}
