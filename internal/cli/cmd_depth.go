package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/depth"
)

func init() {
	Register("depth", "score response depth: thoroughness, detail layers, coverage", runDepth)
}

func runDepth(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay depth", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay depth: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay depth: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "depth", err)
	}

	report := depth.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = depth.FormatJSON(report)
	default:
		output = depth.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "depth", err)
	}

	return ExitPass
}
