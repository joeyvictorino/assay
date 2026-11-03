package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/contextadherence"
)

func init() {
	Register("context-adherence", "score how well responses stick to provided context", runContextAdherence)
}

func runContextAdherence(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay context-adherence", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay context-adherence: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay context-adherence: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "context-adherence", err)
	}

	report := contextadherence.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = contextadherence.FormatJSON(report)
	default:
		output = contextadherence.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "context-adherence", err)
	}

	return ExitPass
}
