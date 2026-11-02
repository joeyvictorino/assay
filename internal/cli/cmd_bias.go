package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/bias"
)

func init() {
	Register("bias", "detect response biases in findings", runBias)
}

func runBias(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay bias", stderr)
	findings := fs.String("findings", "", "path to findings JSON file (required)")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *findings == "" {
		fmt.Fprintln(stderr, "assay bias: --findings is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay bias: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	report, err := bias.AnalyzeBias(*findings)
	if err != nil {
		return fail(stderr, "bias", err)
	}

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fail(stderr, "bias", err)
		}
		output = string(b) + "\n"
	default:
		output = bias.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "bias", err)
	}
	return ExitPass
}
