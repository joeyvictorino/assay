package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/temperature"
)

func init() {
	Register("temperature", "analyze finding severity across temperature settings", runTemperature)
}

func runTemperature(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay temperature", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay temperature: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "temperature", err)
	}

	report := temperature.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = temperature.FormatJSON(report)
	default:
		output = temperature.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "temperature", err)
	}

	return ExitPass
}
