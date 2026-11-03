package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/prioritization"
)

func init() {
	Register("prioritization", "score finding prioritization, triage accuracy, and severity ordering", runPrioritization)
}

func runPrioritization(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prioritization", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay prioritization: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay prioritization: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "prioritization", err)
	}

	report := prioritization.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = prioritization.FormatJSON(report)
	default:
		output = prioritization.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "prioritization", err)
	}

	return ExitPass
}
