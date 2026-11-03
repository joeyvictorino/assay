package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/modellatency"
)

func init() {
	Register("model-latency", "analyze response time distribution per model with histogram buckets", runModelLatency)
}

func runModelLatency(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-latency", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-latency: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "model-latency", err)
	}

	report := modellatency.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = modellatency.FormatJSON(report)
	default:
		output = modellatency.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-latency", err)
	}

	return ExitPass
}
