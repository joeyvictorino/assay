package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/patience"
)

func init() {
	Register("patience", "score patience and thoroughness of model responses", runPatience)
}

func runPatience(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay patience", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay patience: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay patience: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "patience", err)
	}

	report := patience.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = patience.FormatJSON(report)
	default:
		output = patience.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "patience", err)
	}

	return ExitPass
}
