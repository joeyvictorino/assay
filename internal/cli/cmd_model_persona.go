package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/persona"
)

func init() {
	Register("model-persona", "analyze model persona consistency across findings", runModelPersona)
}

func runModelPersona(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-persona", stderr)
	findingsPath := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-persona: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	findings, err := readFindings(*findingsPath, nil)
	if err != nil {
		return fail(stderr, "model-persona", err)
	}

	report := persona.Analyze(findings)

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
		return fail(stderr, "model-persona", err)
	}

	return ExitPass
}
