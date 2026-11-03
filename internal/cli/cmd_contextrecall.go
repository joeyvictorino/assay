package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/contextrecall"
)

func init() {
	Register("context-recall", "score context recall accuracy and citation quality", runContextRecall)
}

func runContextRecall(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay context-recall", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay context-recall: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay context-recall: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "context-recall", err)
	}

	report := contextrecall.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = contextrecall.FormatJSON(report)
	default:
		output = contextrecall.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "context-recall", err)
	}

	return ExitPass
}
