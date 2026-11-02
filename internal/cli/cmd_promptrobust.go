package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/promptrobust"
)

func init() {
	Register("prompt-robustness", "measure prompt perturbation robustness across models", runPromptRobust)
}

func runPromptRobust(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-robustness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay prompt-robustness: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay prompt-robustness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "prompt-robustness", err)
	}

	report := promptrobust.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = promptrobust.FormatJSON(report)
	default:
		output = promptrobust.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "prompt-robustness", err)
	}

	return ExitPass
}
