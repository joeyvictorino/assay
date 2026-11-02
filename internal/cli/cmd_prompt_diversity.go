package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/promptdiversity"
)

func init() {
	Register("prompt-diversity", "score prompt template diversity across findings", runPromptDiversity)
}

func runPromptDiversity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-diversity", stderr)
	findingsPath := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay prompt-diversity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	findings, err := readFindings(*findingsPath, nil)
	if err != nil {
		return fail(stderr, "prompt-diversity", err)
	}

	report := promptdiversity.Analyze(findings)

	var output string
	switch *format {
	case "json":
		output = promptdiversity.FormatJSON(report)
	default:
		output = promptdiversity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "prompt-diversity", err)
	}

	return ExitPass
}
