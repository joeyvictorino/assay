package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/analogical"
)

func init() {
	Register("analogical", "score analogical reasoning quality, mapping precision, and transfer quality", runAnalogical)
}

func runAnalogical(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay analogical", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay analogical: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay analogical: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "analogical", err)
	}

	report := analogical.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = analogical.FormatJSON(report)
	default:
		output = analogical.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "analogical", err)
	}

	return ExitPass
}
