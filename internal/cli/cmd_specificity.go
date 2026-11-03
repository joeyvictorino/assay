package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/specificity"
)

func init() {
	Register("specificity", "analyze response specificity scoring", runSpecificity)
}

func runSpecificity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay specificity", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay specificity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay specificity: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "specificity", err)
	}

	report := specificity.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = specificity.FormatJSON(report)
	default:
		output = specificity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "specificity", err)
	}

	return ExitPass
}
