package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/classcov"
)

func init() {
	Register("class-coverage", "analyze vulnerability class coverage across findings", runClassCov)
}

func runClassCov(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay class-coverage", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay class-coverage: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "class-coverage", err)
	}

	report := classcov.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = classcov.FormatJSON(report)
	default:
		output = classcov.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "class-coverage", err)
	}

	return ExitPass
}
