package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/ambiguity"
)

func init() {
	Register("ambiguity", "analyze model handling of ambiguous prompts and inputs", runAmbiguity)
}

func runAmbiguity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay ambiguity", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay ambiguity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay ambiguity: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "ambiguity", err)
	}

	report := ambiguity.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = ambiguity.FormatJSON(report)
	default:
		output = ambiguity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "ambiguity", err)
	}

	return ExitPass
}
