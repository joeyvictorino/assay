package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/relevance"
)

func init() {
	Register("relevance", "analyze response relevance to prompts", runRelevance)
}

func runRelevance(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay relevance", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay relevance: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay relevance: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "relevance", err)
	}

	report := relevance.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = relevance.FormatJSON(report)
	default:
		output = relevance.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "relevance", err)
	}

	return ExitPass
}
