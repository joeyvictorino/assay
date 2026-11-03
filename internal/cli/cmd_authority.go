package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/authority"
)

func init() {
	Register("authority", "score response authority, expertise signals, and domain knowledge", runAuthority)
}

func runAuthority(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay authority", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay authority: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay authority: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "authority", err)
	}

	report := authority.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = authority.FormatJSON(report)
	default:
		output = authority.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "authority", err)
	}

	return ExitPass
}
