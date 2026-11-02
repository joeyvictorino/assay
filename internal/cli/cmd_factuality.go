package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/factuality"
)

func init() {
	Register("factuality", "validate factual accuracy of model responses", runFactuality)
}

func runFactuality(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay factuality", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay factuality: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay factuality: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "factuality", err)
	}

	report := factuality.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = factuality.FormatJSON(report)
	default:
		output = factuality.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "factuality", err)
	}

	return ExitPass
}
