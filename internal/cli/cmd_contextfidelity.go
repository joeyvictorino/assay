package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/contextfidelity"
)

func init() {
	Register("context-fidelity", "score context fidelity, source attribution, and hallucination resistance", runContextFidelity)
}

func runContextFidelity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay context-fidelity", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay context-fidelity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay context-fidelity: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "context-fidelity", err)
	}

	report := contextfidelity.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = contextfidelity.FormatJSON(report)
	default:
		output = contextfidelity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "context-fidelity", err)
	}

	return ExitPass
}
