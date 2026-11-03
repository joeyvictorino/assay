package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/safetyboundary"
)

func init() {
	Register("safety-boundary", "analyze safety boundary probing across findings", runSafetyBoundary)
}

func runSafetyBoundary(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay safety-boundary", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay safety-boundary: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "safety-boundary", err)
	}

	report := safetyboundary.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = safetyboundary.FormatJSON(report)
	default:
		output = safetyboundary.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "safety-boundary", err)
	}

	return ExitPass
}
