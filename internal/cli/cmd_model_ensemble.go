package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/modelensemble"
)

func init() {
	Register("model-ensemble", "multi-model ensemble voting analysis", runModelEnsemble)
}

func runModelEnsemble(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-ensemble", stderr)
	input := fs.String("findings", "", "path to findings JSON (required, - for stdin)")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay model-ensemble: --findings is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-ensemble: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	findings, err := readFindings(*input, nil)
	if err != nil {
		return fail(stderr, "model-ensemble", err)
	}

	report := modelensemble.Analyze(findings)

	var output string
	switch *format {
	case "json":
		output = modelensemble.FormatJSON(report)
	default:
		output = modelensemble.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-ensemble", err)
	}
	return ExitPass
}
