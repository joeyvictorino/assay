package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/abstraction"
)

func init() {
	Register("abstraction", "score response abstraction quality, level appropriateness, and scope control", runAbstraction)
}

func runAbstraction(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay abstraction", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay abstraction: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay abstraction: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "abstraction", err)
	}

	report := abstraction.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = abstraction.FormatJSON(report)
	default:
		output = abstraction.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "abstraction", err)
	}

	return ExitPass
}
