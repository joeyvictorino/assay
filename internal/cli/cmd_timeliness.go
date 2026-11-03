package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/timeliness"
)

func init() {
	Register("timeliness", "score response timeliness, speed of delivery, and update freshness", runTimeliness)
}

func runTimeliness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay timeliness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay timeliness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay timeliness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "timeliness", err)
	}

	report := timeliness.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = timeliness.FormatJSON(report)
	default:
		output = timeliness.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "timeliness", err)
	}

	return ExitPass
}
