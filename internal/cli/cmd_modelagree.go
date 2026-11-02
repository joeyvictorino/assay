package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/modelagree"
)

func init() {
	Register("model-agreement", "inter-model agreement scoring across shared prompts", runModelAgreement)
}

func runModelAgreement(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-agreement", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-agreement: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "model-agreement", err)
	}

	r := modelagree.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = modelagree.FormatJSON(r)
	default:
		output = modelagree.FormatText(r)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-agreement", err)
	}

	return ExitPass
}
