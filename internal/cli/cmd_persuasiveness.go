package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/persuasiveness"
)

func init() {
	Register("persuasiveness", "score argument strength, evidence use, and rhetorical skill", runPersuasiveness)
}

func runPersuasiveness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay persuasiveness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay persuasiveness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay persuasiveness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "persuasiveness", err)
	}

	report := persuasiveness.Analyze(data)
	var output string
	switch *format {
	case "json":
		output = persuasiveness.FormatJSON(report)
	default:
		output = persuasiveness.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "persuasiveness", err)
	}
	return ExitPass
}
