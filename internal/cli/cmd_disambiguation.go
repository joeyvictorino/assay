package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/disambiguation"
)

func init() {
	Register("disambiguation", "analyze model disambiguation quality scoring", runDisambiguation)
}

func runDisambiguation(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay disambiguation", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay disambiguation: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay disambiguation: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "disambiguation", err)
	}

	report := disambiguation.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = disambiguation.FormatJSON(report)
	default:
		output = disambiguation.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "disambiguation", err)
	}

	return ExitPass
}
