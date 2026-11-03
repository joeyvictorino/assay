package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/temporal"
)

func init() {
	Register("temporal", "score temporal reasoning quality, chronological accuracy, and sequence understanding", runTemporal)
}

func runTemporal(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay temporal", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay temporal: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay temporal: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "temporal", err)
	}

	report := temporal.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = temporal.FormatJSON(report)
	default:
		output = temporal.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "temporal", err)
	}

	return ExitPass
}
