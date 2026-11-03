package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/sarcasm"
)

func init() {
	Register("sarcasm", "score sarcasm, dismissiveness, and passive-aggressive tone in model responses", runSarcasm)
}

func runSarcasm(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay sarcasm", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay sarcasm: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay sarcasm: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "sarcasm", err)
	}

	report := sarcasm.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = sarcasm.FormatJSON(report)
	default:
		output = sarcasm.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "sarcasm", err)
	}

	return ExitPass
}
