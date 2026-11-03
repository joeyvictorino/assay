package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/tone"
)

func init() {
	Register("tone", "score response tone appropriateness, warmth, and professionalism", runTone)
}

func runTone(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay tone", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay tone: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay tone: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "tone", err)
	}

	report := tone.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = tone.FormatJSON(report)
	default:
		output = tone.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "tone", err)
	}

	return ExitPass
}
