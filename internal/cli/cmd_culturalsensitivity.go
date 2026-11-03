package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/culturalsensitivity"
)

func init() {
	Register("culturalsensitivity", "score cultural awareness, inclusive language, and bias avoidance", runCulturalSensitivity)
}

func runCulturalSensitivity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay culturalsensitivity", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay culturalsensitivity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay culturalsensitivity: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "culturalsensitivity", err)
	}

	report := culturalsensitivity.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = culturalsensitivity.FormatJSON(report)
	default:
		output = culturalsensitivity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "culturalsensitivity", err)
	}

	return ExitPass
}
