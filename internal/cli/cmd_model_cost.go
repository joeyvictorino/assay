package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/modelcost"
)

func init() {
	Register("model-cost", "cost-per-finding analysis across models", runModelCost)
}

func runModelCost(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-cost", stderr)
	input := fs.String("findings", "", "path to findings JSON (required, - for stdin)")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay model-cost: --findings is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-cost: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	findings, err := readFindings(*input, nil)
	if err != nil {
		return fail(stderr, "model-cost", err)
	}

	report := modelcost.Analyze(findings)

	var output string
	switch *format {
	case "json":
		output = modelcost.FormatJSON(report)
	default:
		output = modelcost.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-cost", err)
	}
	return ExitPass
}
