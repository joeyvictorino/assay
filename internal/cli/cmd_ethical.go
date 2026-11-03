package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/ethical"
)

func init() {
	Register("ethical", "score ethical reasoning quality: principles, stakeholders, consequences, fairness", runEthical)
}

func runEthical(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay ethical", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay ethical: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay ethical: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "ethical", err)
	}

	report := ethical.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = ethical.FormatJSON(report)
	default:
		output = ethical.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "ethical", err)
	}

	return ExitPass
}
