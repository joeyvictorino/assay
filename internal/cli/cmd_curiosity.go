package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/curiosity"
)

func init() {
	Register("curiosity", "score intellectual curiosity, inquiry depth, and open-mindedness", runCuriosity)
}

func runCuriosity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay curiosity", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay curiosity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay curiosity: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "curiosity", err)
	}

	report := curiosity.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = curiosity.FormatJSON(report)
	default:
		output = curiosity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "curiosity", err)
	}

	return ExitPass
}
