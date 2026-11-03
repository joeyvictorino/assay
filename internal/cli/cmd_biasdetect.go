package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/biasdetect"
)

func init() {
	Register("bias-detect", "detect bias patterns in model responses", runBiasDetect)
}

func runBiasDetect(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay bias-detect", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay bias-detect: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay bias-detect: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "bias-detect", err)
	}

	report := biasdetect.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = biasdetect.FormatJSON(report)
	default:
		output = biasdetect.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "bias-detect", err)
	}

	return ExitPass
}
