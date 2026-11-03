package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/outputconsistency"
)

func init() {
	Register("output-consistency", "score output consistency across repeated model runs", runOutputConsistency)
}

func runOutputConsistency(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay output-consistency", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay output-consistency: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay output-consistency: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "output-consistency", err)
	}

	report := outputconsistency.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = outputconsistency.FormatJSON(report)
	default:
		output = outputconsistency.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "output-consistency", err)
	}

	return ExitPass
}
