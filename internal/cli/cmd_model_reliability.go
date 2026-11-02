package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/reliability"
)

func init() {
	Register("model-reliability", "measure model response reliability metrics", runModelReliability)
}

func runModelReliability(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-reliability", stderr)
	findingsPath := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-reliability: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	findings, err := readFindings(*findingsPath, nil)
	if err != nil {
		return fail(stderr, "model-reliability", err)
	}

	report := reliability.Analyze(findings)

	var output string
	switch *format {
	case "json":
		output = reliability.FormatJSON(report)
	default:
		output = reliability.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-reliability", err)
	}

	return ExitPass
}
