package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/brevity"
)

func init() {
	Register("brevity", "score response brevity, filler ratio, and word efficiency", runBrevity)
}

func runBrevity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay brevity", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay brevity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay brevity: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "brevity", err)
	}

	report := brevity.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = brevity.FormatJSON(report)
	default:
		output = brevity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "brevity", err)
	}

	return ExitPass
}
