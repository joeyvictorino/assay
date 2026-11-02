package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/contextwin"
)

func init() {
	Register("context-window", "analyze context window utilization efficiency", runContextWindow)
}

func runContextWindow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay context-window", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay context-window: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "context-window", err)
	}

	report := contextwin.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = contextwin.FormatJSON(report)
	default:
		output = contextwin.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "context-window", err)
	}

	return ExitPass
}
