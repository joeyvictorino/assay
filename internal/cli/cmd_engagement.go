package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/engagement"
)

func init() {
	Register("engagement", "score response engagement and interactivity quality", runEngagement)
}

func runEngagement(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay engagement", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay engagement: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay engagement: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "engagement", err)
	}

	report := engagement.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = engagement.FormatJSON(report)
	default:
		output = engagement.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "engagement", err)
	}

	return ExitPass
}
