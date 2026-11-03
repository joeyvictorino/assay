package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/concreteness"
)

func init() {
	Register("concreteness", "score response concreteness, specificity, and example usage", runConcreteness)
}

func runConcreteness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay concreteness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay concreteness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay concreteness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "concreteness", err)
	}

	report := concreteness.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = concreteness.FormatJSON(report)
	default:
		output = concreteness.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "concreteness", err)
	}

	return ExitPass
}
