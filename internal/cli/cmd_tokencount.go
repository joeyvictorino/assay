package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/tokencount"
)

func init() {
	Register("token-count", "estimate token usage per model, prompt, and checker", runTokenCount)
}

func runTokenCount(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay token-count", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay token-count: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "token-count", err)
	}

	report := tokencount.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = tokencount.FormatJSON(report)
	default:
		output = tokencount.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "token-count", err)
	}

	return ExitPass
}
