package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/analogy"
)

func init() {
	Register("analogy", "score analogy quality, relevance, clarity, and novelty", runAnalogy)
}

func runAnalogy(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay analogy", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay analogy: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay analogy: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "analogy", err)
	}

	report := analogy.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = analogy.FormatJSON(report)
	default:
		output = analogy.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "analogy", err)
	}

	return ExitPass
}
