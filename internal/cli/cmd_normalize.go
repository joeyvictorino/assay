package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/normalize"
)

func init() {
	Register("normalize", "normalize and deduplicate findings into a canonical schema", runNormalize)
}

func runNormalize(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay normalize", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay normalize: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay normalize: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "normalize", err)
	}

	report := normalize.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = normalize.FormatJSON(report)
	default:
		output = normalize.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "normalize", err)
	}

	return ExitPass
}
