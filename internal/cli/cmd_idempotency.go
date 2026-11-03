package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/idempotency"
)

func init() {
	Register("idempotency", "score model output consistency across repeated runs", runIdempotency)
}

func runIdempotency(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay idempotency", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay idempotency: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay idempotency: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "idempotency", err)
	}

	report := idempotency.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = idempotency.FormatJSON(report)
	default:
		output = idempotency.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "idempotency", err)
	}

	return ExitPass
}
