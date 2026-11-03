package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/promptcat"
)

func init() {
	Register("prompt-catalog", "prompt template catalog with usage and duplicate detection", runPromptCatalog)
}

func runPromptCatalog(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-catalog", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay prompt-catalog: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "prompt-catalog", err)
	}

	cr := promptcat.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = promptcat.FormatJSON(cr)
	default:
		output = promptcat.FormatText(cr)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "prompt-catalog", err)
	}

	return ExitPass
}
