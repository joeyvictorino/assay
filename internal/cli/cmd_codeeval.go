package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/codeeval"
)

func init() {
	Register("code-eval", "evaluate code generation quality", runCodeEval)
}

func runCodeEval(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay code-eval", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay code-eval: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay code-eval: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "code-eval", err)
	}

	report := codeeval.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = codeeval.FormatJSON(report)
	default:
		output = codeeval.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "code-eval", err)
	}

	return ExitPass
}
