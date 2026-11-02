package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/instructionconflict"
)

func init() {
	Register("instruction-conflict", "detect instruction hierarchy violations and prompt conflicts", runInstructionConflict)
}

func runInstructionConflict(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay instruction-conflict", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay instruction-conflict: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay instruction-conflict: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "instruction-conflict", err)
	}

	report := instructionconflict.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = instructionconflict.FormatJSON(report)
	default:
		output = instructionconflict.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "instruction-conflict", err)
	}

	return ExitPass
}
