package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/watermark"
)

func init() {
	Register("watermark", "detect AI watermark patterns in model output", runWatermark)
}

func runWatermark(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay watermark", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay watermark: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay watermark: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "watermark", err)
	}

	report := watermark.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = watermark.FormatJSON(report)
	default:
		output = watermark.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "watermark", err)
	}

	return ExitPass
}
