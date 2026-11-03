package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/logiccheck"
)

func init() {
	Register("logic-check", "evaluate logical soundness of model responses", runLogicCheck)
}

func runLogicCheck(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay logic-check", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay logic-check: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay logic-check: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "logic-check", err)
	}

	report := logiccheck.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = logiccheck.FormatJSON(report)
	default:
		output = logiccheck.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "logic-check", err)
	}

	return ExitPass
}
