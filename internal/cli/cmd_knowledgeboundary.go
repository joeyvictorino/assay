package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/knowledgeboundary"
)

func init() {
	Register("knowledgeboundary", "analyze model knowledge boundary awareness", runKnowledgeboundary)
}

func runKnowledgeboundary(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay knowledgeboundary", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay knowledgeboundary: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay knowledgeboundary: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "knowledgeboundary", err)
	}

	report := knowledgeboundary.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = knowledgeboundary.FormatJSON(report)
	default:
		output = knowledgeboundary.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "knowledgeboundary", err)
	}

	return ExitPass
}
