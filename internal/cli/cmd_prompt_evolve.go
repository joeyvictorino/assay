package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/promptevolve"
)

func init() {
	Register("prompt-evolve", "track prompt mutation effectiveness over runs", runPromptEvolve)
}

func runPromptEvolve(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-evolve", stderr)
	input := fs.String("findings", "", "path to findings JSON (required, - for stdin)")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *input == "" {
		fmt.Fprintln(stderr, "assay prompt-evolve: --findings is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay prompt-evolve: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	findings, err := readFindings(*input, nil)
	if err != nil {
		return fail(stderr, "prompt-evolve", err)
	}

	report := promptevolve.Analyze(findings)

	var output string
	switch *format {
	case "json":
		output = promptevolve.FormatJSON(report)
	default:
		output = promptevolve.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "prompt-evolve", err)
	}
	return ExitPass
}
