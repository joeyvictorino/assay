package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/alignment"
)

func init() {
	Register("alignment", "measure model alignment scores across safety dimensions", runAlignment)
}

func runAlignment(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay alignment", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay alignment: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay alignment: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "alignment", err)
	}

	report := alignment.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = alignment.FormatJSON(report)
	default:
		output = alignment.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "alignment", err)
	}

	return ExitPass
}
