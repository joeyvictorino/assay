package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/courtesy"
)

func init() {
	Register("courtesy", "score response courtesy: politeness, acknowledgment vs rudeness, dismissiveness", runCourtesy)
}

func runCourtesy(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay courtesy", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay courtesy: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay courtesy: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "courtesy", err)
	}

	report := courtesy.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = courtesy.FormatJSON(report)
	default:
		output = courtesy.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "courtesy", err)
	}

	return ExitPass
}
