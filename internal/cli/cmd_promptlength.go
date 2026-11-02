package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/promptlength"
)

func init() {
	Register("prompt-length", "analyze prompt length impact on finding quality", runPromptLength)
}

func runPromptLength(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-length", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay prompt-length: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "prompt-length", err)
	}

	report := promptlength.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = promptlength.FormatJSON(report)
	default:
		output = promptlength.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "prompt-length", err)
	}

	return ExitPass
}
