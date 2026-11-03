package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/emotional_range"
)

func init() {
	Register("emotional-range", "score response emotional range, empathy, and tone variety", runEmotionalRange)
}

func runEmotionalRange(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay emotional-range", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay emotional-range: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay emotional-range: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "emotional-range", err)
	}

	report := emotional_range.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = emotional_range.FormatJSON(report)
	default:
		output = emotional_range.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "emotional-range", err)
	}

	return ExitPass
}
