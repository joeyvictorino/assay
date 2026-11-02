package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/reportcard"
)

func init() {
	Register("report-card", "model evaluation report card with letter grades", runReportCard)
}

func runReportCard(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay report-card", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay report-card: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "report-card", err)
	}

	rc := reportcard.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = reportcard.FormatJSON(rc)
	default:
		output = reportcard.FormatText(rc)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "report-card", err)
	}

	return ExitPass
}
