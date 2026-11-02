package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/diversity"
)

func init() {
	Register("diversity", "score response diversity across findings", runDiversity)
}

func runDiversity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay diversity", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to this file (\"-\" for stdout, default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *findings == "" {
		fmt.Fprintln(stderr, "assay diversity: --findings is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay diversity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "diversity", err)
	}

	report := diversity.Analyze(ff)

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fail(stderr, "diversity", err)
		}
		output = string(b) + "\n"
	default:
		output = diversity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "diversity", err)
	}
	return ExitPass
}
