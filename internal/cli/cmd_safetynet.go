package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/safetynet"
)

func init() {
	Register("safetynet", "score safety boundary enforcement and policy compliance", runSafetynet)
}

func runSafetynet(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay safetynet", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay safetynet: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay safetynet: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "safetynet", err)
	}

	report := safetynet.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = safetynet.FormatJSON(report)
	default:
		output = safetynet.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "safetynet", err)
	}

	return ExitPass
}
