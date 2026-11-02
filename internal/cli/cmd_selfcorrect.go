package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/selfcorrect"
)

func init() {
	Register("selfcorrect", "analyze model self-correction behavior and accuracy improvement", runSelfCorrect)
}

func runSelfCorrect(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay selfcorrect", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay selfcorrect: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay selfcorrect: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "selfcorrect", err)
	}

	report := selfcorrect.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = selfcorrect.FormatJSON(report)
	default:
		output = selfcorrect.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "selfcorrect", err)
	}

	return ExitPass
}
