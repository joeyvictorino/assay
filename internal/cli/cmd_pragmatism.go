package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/pragmatism"
)

func init() {
	Register("pragmatism", "score response pragmatism, actionability, and practical usefulness", runPragmatism)
}

func runPragmatism(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay pragmatism", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay pragmatism: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay pragmatism: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "pragmatism", err)
	}

	report := pragmatism.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = pragmatism.FormatJSON(report)
	default:
		output = pragmatism.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "pragmatism", err)
	}

	return ExitPass
}
