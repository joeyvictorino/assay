package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/sourcing"
)

func init() {
	Register("sourcing", "score citation quality, source attribution, and reference traceability", runSourcing)
}

func runSourcing(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay sourcing", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay sourcing: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay sourcing: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "sourcing", err)
	}

	report := sourcing.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = sourcing.FormatJSON(report)
	default:
		output = sourcing.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "sourcing", err)
	}

	return ExitPass
}
