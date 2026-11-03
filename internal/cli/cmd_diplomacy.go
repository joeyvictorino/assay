package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/diplomacy"
)

func init() {
	Register("diplomacy", "score diplomatic communication, tact, and balanced perspective", runDiplomacy)
}

func runDiplomacy(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay diplomacy", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay diplomacy: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay diplomacy: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "diplomacy", err)
	}

	report := diplomacy.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = diplomacy.FormatJSON(report)
	default:
		output = diplomacy.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "diplomacy", err)
	}

	return ExitPass
}
