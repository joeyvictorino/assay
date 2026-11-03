package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/proactivity"
)

func init() {
	Register("proactivity", "score proactive guidance: warnings, prevention, alternatives vs reactive fixes", runProactivity)
}

func runProactivity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay proactivity", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay proactivity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay proactivity: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "proactivity", err)
	}

	report := proactivity.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = proactivity.FormatJSON(report)
	default:
		output = proactivity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "proactivity", err)
	}

	return ExitPass
}
