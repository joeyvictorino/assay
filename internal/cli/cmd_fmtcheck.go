package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/fmtcheck"
)

func init() {
	Register("format-compliance", "score output format adherence across findings", runFormatCompliance)
}

func runFormatCompliance(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay format-compliance", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay format-compliance: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "format-compliance", err)
	}

	report := fmtcheck.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = fmtcheck.FormatJSON(report)
	default:
		output = fmtcheck.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "format-compliance", err)
	}

	return ExitPass
}
