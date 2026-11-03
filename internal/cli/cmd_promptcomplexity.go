package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/promptcomplexity"
)

func init() {
	Register("prompt-complexity", "score prompt structural complexity", runPromptComplexity)
}

func runPromptComplexity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-complexity", stderr)
	input := fs.String("input", "", "path to prompt text file (required)")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay prompt-complexity: --input is required")
		fs.Usage()
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		return fail(stderr, "prompt-complexity", err)
	}

	report := promptcomplexity.Analyze(string(data))

	var output string
	switch *format {
	case "json":
		output = promptcomplexity.FormatJSON(report)
	default:
		output = promptcomplexity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "prompt-complexity", err)
	}
	return ExitPass
}
