package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/refusal"
)

func init() {
	Register("refusal-rate", "analyze model refusal frequency across findings", runRefusalRate)
}

func runRefusalRate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay refusal-rate", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay refusal-rate: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "refusal-rate", err)
	}

	report := refusal.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = refusal.FormatJSON(report)
	default:
		output = refusal.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "refusal-rate", err)
	}

	return ExitPass
}
