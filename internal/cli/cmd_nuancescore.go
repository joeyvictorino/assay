package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/nuancescore"
)

func init() {
	Register("nuance-score", "score response nuance and perspective handling", runNuanceScore)
}

func runNuanceScore(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay nuance-score", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay nuance-score: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay nuance-score: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "nuance-score", err)
	}

	report := nuancescore.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = nuancescore.FormatJSON(report)
	default:
		output = nuancescore.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "nuance-score", err)
	}

	return ExitPass
}
