package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/answercompleteness"
)

func init() {
	Register("answer-completeness", "score answer completeness for multi-part questions", runAnswerCompleteness)
}

func runAnswerCompleteness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay answer-completeness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay answer-completeness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay answer-completeness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "answer-completeness", err)
	}

	report := answercompleteness.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = answercompleteness.FormatJSON(report)
	default:
		output = answercompleteness.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "answer-completeness", err)
	}

	return ExitPass
}
