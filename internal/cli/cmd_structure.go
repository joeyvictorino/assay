package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/structure"
)

func init() {
	Register("structure", "score structural organization: headings, lists, code blocks vs wall-of-text", runStructure)
}

func runStructure(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay structure", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay structure: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay structure: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "structure", err)
	}

	report := structure.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = structure.FormatJSON(report)
	default:
		output = structure.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "structure", err)
	}

	return ExitPass
}
