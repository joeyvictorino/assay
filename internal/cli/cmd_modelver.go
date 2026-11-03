package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/modelver"
)

func init() {
	Register("model-version", "track model versions across runs and detect changes", runModelVersion)
}

func runModelVersion(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-version", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-version: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "model-version", err)
	}

	report := modelver.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = modelver.FormatJSON(report)
	default:
		output = modelver.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-version", err)
	}

	return ExitPass
}
