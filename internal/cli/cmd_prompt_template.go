package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/prompttemplate"
)

func init() {
	Register("prompt-template", "analyze a prompt template library", runPromptTemplate)
}

func runPromptTemplate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-template", stderr)
	input := fs.String("input", "", "path to prompt template file (required)")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay prompt-template: --input is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay prompt-template: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		return fail(stderr, "prompt-template", err)
	}

	lib, err := prompttemplate.Parse(string(data))
	if err != nil {
		return fail(stderr, "prompt-template", err)
	}

	report := prompttemplate.Analyze(lib)

	var output string
	switch *format {
	case "json":
		output = prompttemplate.FormatJSON(report)
	default:
		output = prompttemplate.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "prompt-template", err)
	}
	return ExitPass
}
