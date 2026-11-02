package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/multimodal"
)

func init() {
	Register("multimodal", "analyze multi-modal response consistency", runMultimodal)
}

func runMultimodal(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay multimodal", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay multimodal: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay multimodal: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "multimodal", err)
	}

	report := multimodal.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = multimodal.FormatJSON(report)
	default:
		output = multimodal.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "multimodal", err)
	}

	return ExitPass
}
