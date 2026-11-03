package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/edgecase"
)

func init() {
	Register("edgecase", "score model edge case handling: boundary, null, overflow, empty input, and ambiguity accuracy", runEdgecase)
}

func runEdgecase(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay edgecase", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay edgecase: unknown --format %q (text | json)\n", *format)
		return ExitError
	}
	if *findings == "" {
		fmt.Fprintln(stderr, "assay edgecase: --findings is required")
		return ExitError
	}
	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "edgecase", err)
	}
	report := edgecase.Analyze(data)
	var output string
	switch *format {
	case "json":
		output = edgecase.FormatJSON(report)
	default:
		output = edgecase.FormatText(report)
	}
	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "edgecase", err)
	}
	return ExitPass
}
