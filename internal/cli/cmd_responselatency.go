package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/responselatency"
)

func init() {
	Register("response-latency", "analyze response latency distribution and benchmarks", runResponseLatency)
}

func runResponseLatency(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay response-latency", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay response-latency: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay response-latency: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "response-latency", err)
	}

	report := responselatency.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = responselatency.FormatJSON(report)
	default:
		output = responselatency.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "response-latency", err)
	}

	return ExitPass
}
