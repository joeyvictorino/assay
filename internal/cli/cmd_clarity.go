package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/clarity"
)

func init() {
	Register("clarity", "score response clarity: logical flow, structure, ambiguity", runClarity)
}

func runClarity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay clarity", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay clarity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay clarity: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "clarity", err)
	}

	report := clarity.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = clarity.FormatJSON(report)
	default:
		output = clarity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "clarity", err)
	}

	return ExitPass
}
