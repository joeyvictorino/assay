package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/metacognition"
)

func init() {
	Register("metacognition", "score response self-awareness, reasoning transparency, and uncertainty calibration", runMetacognition)
}

func runMetacognition(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay metacognition", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay metacognition: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay metacognition: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "metacognition", err)
	}

	report := metacognition.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = metacognition.FormatJSON(report)
	default:
		output = metacognition.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "metacognition", err)
	}

	return ExitPass
}
