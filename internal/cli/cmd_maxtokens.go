package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/maxtokens"
)

func init() {
	Register("max-tokens", "analyze finding severity across max-token settings", runMaxTokens)
}

func runMaxTokens(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay max-tokens", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay max-tokens: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "max-tokens", err)
	}

	report := maxtokens.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = maxtokens.FormatJSON(report)
	default:
		output = maxtokens.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "max-tokens", err)
	}

	return ExitPass
}
