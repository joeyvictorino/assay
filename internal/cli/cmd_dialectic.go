package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/dialectic"
)

func init() {
	Register("dialectic", "score dialectical reasoning quality: thesis, antithesis, synthesis, nuance", runDialectic)
}

func runDialectic(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay dialectic", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay dialectic: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay dialectic: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "dialectic", err)
	}

	report := dialectic.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = dialectic.FormatJSON(report)
	default:
		output = dialectic.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "dialectic", err)
	}

	return ExitPass
}
