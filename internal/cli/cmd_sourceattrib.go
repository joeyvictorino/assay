package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/sourceattrib"
)

func init() {
	Register("source-attribution", "score model evidence and source attribution quality", runSourceAttrib)
}

func runSourceAttrib(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay source-attribution", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay source-attribution: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay source-attribution: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "source-attribution", err)
	}

	report := sourceattrib.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = sourceattrib.FormatJSON(report)
	default:
		output = sourceattrib.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "source-attribution", err)
	}

	return ExitPass
}
