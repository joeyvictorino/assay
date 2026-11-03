package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/succinctness"
)

func init() {
	Register("succinctness", "analyze response succinctness and verbosity", runSuccinctness)
}

func runSuccinctness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay succinctness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay succinctness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay succinctness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "succinctness", err)
	}

	report := succinctness.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = succinctness.FormatJSON(report)
	default:
		output = succinctness.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "succinctness", err)
	}

	return ExitPass
}
