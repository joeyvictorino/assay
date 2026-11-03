package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/metaphor"
)

func init() {
	Register("metaphor", "score metaphor quality: aptness, novelty, clarity, extension, and consistency", runMetaphor)
}

func runMetaphor(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay metaphor", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay metaphor: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay metaphor: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "metaphor", err)
	}

	report := metaphor.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = metaphor.FormatJSON(report)
	default:
		output = metaphor.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "metaphor", err)
	}

	return ExitPass
}
