package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/persona"
)

func init() {
	Register("persona", "score response persona consistency and character adherence", runPersona)
}

func runPersona(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay persona", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay persona: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay persona: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "persona", err)
	}

	report := persona.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = persona.FormatJSON(report)
	default:
		output = persona.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "persona", err)
	}

	return ExitPass
}
