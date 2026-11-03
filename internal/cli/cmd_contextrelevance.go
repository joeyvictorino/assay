package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/contextrelevance"
)

func init() {
	Register("contextrelevance", "score response context relevance, topic alignment, and focus", runContextrelevance)
}

func runContextrelevance(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay contextrelevance", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay contextrelevance: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay contextrelevance: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "contextrelevance", err)
	}

	report := contextrelevance.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = contextrelevance.FormatJSON(report)
	default:
		output = contextrelevance.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "contextrelevance", err)
	}

	return ExitPass
}
