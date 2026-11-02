package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/promptrisk"
)

func init() {
	Register("prompt-risk", "prompt risk classification and scoring", runPromptRisk)
}

func runPromptRisk(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-risk", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay prompt-risk: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "prompt-risk", err)
	}

	r := promptrisk.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = promptrisk.FormatJSON(r)
	default:
		output = promptrisk.FormatText(r)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "prompt-risk", err)
	}

	return ExitPass
}
