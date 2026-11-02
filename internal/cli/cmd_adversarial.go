package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/adversarial"
)

func init() {
	Register("adversarial", "test adversarial prompt robustness across findings", runAdversarial)
}

func runAdversarial(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay adversarial", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay adversarial: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "adversarial", err)
	}

	report := adversarial.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = adversarial.FormatJSON(report)
	default:
		output = adversarial.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "adversarial", err)
	}

	return ExitPass
}
