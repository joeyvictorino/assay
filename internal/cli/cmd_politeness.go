package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/politeness"
)

func init() {
	Register("politeness", "score politeness, courtesy, and respectful communication", runPoliteness)
}

func runPoliteness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay politeness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay politeness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay politeness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "politeness", err)
	}

	report := politeness.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = politeness.FormatJSON(report)
	default:
		output = politeness.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "politeness", err)
	}

	return ExitPass
}
