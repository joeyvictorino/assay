package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/neutrality"
)

func init() {
	Register("neutrality", "score response neutrality and bias detection", runNeutrality)
}

func runNeutrality(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay neutrality", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay neutrality: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay neutrality: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "neutrality", err)
	}

	report := neutrality.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = neutrality.FormatJSON(report)
	default:
		output = neutrality.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "neutrality", err)
	}

	return ExitPass
}
