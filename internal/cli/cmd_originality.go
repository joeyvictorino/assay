package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/originality"
)

func init() {
	Register("originality", "score response originality and novelty", runOriginality)
}

func runOriginality(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay originality", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay originality: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay originality: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "originality", err)
	}

	report := originality.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = originality.FormatJSON(report)
	default:
		output = originality.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "originality", err)
	}

	return ExitPass
}
