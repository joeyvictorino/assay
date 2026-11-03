package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/summarization"
)

func init() {
	Register("summarization", "score summarization quality and coverage", runSummarization)
}

func runSummarization(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay summarization", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay summarization: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay summarization: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "summarization", err)
	}

	report := summarization.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = summarization.FormatJSON(report)
	default:
		output = summarization.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "summarization", err)
	}

	return ExitPass
}
