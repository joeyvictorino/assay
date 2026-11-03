package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/rigor"
)

func init() {
	Register("rigor", "score analytical rigor: evidence, logic, citations vs handwaving", runRigor)
}

func runRigor(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay rigor", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay rigor: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay rigor: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "rigor", err)
	}

	report := rigor.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = rigor.FormatJSON(report)
	default:
		output = rigor.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "rigor", err)
	}

	return ExitPass
}
