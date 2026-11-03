package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/inferential"
)

func init() {
	Register("inferential", "score inferential reasoning quality including deductive, inductive, and abductive reasoning", runInferential)
}

func runInferential(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay inferential", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay inferential: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay inferential: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "inferential", err)
	}

	report := inferential.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = inferential.FormatJSON(report)
	default:
		output = inferential.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "inferential", err)
	}

	return ExitPass
}
