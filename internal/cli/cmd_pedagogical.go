package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/pedagogical"
)

func init() {
	Register("pedagogical", "score pedagogical quality, explanation depth, and scaffolding", runPedagogical)
}

func runPedagogical(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay pedagogical", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay pedagogical: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay pedagogical: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "pedagogical", err)
	}

	report := pedagogical.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = pedagogical.FormatJSON(report)
	default:
		output = pedagogical.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "pedagogical", err)
	}

	return ExitPass
}
