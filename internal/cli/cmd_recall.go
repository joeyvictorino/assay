package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/recall"
)

func init() {
	Register("recall", "score response recall, completeness, and topic coverage", runRecall)
}

func runRecall(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay recall", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay recall: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay recall: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "recall", err)
	}

	report := recall.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = recall.FormatJSON(report)
	default:
		output = recall.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "recall", err)
	}

	return ExitPass
}
