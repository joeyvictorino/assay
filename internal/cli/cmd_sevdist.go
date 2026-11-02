package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/sevdist"
)

func init() {
	Register("severity-dist", "analyze finding severity distribution across models", runSevDist)
}

func runSevDist(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay severity-dist", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay severity-dist: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "severity-dist", err)
	}

	report := sevdist.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = sevdist.FormatJSON(report)
	default:
		output = sevdist.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "severity-dist", err)
	}

	return ExitPass
}
