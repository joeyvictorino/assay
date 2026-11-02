package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/stability"
)

func init() {
	Register("stability", "measure prompt stability across repeated runs", runStability)
}

func runStability(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay stability", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	minRuns := fs.Int("min-runs", 3, "minimum runs per group for stability analysis")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay stability: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "stability", err)
	}

	results := stability.Analyze(ff, *minRuns)

	var output string
	switch *format {
	case "json":
		output = stability.FormatJSON(results)
	default:
		output = stability.FormatText(results)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "stability", err)
	}

	return ExitPass
}
