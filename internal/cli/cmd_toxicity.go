package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/toxicity"
)

func init() {
	Register("toxicity", "score model output toxicity across content categories", runToxicity)
}

func runToxicity(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay toxicity", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay toxicity: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay toxicity: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "toxicity", err)
	}

	report := toxicity.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = toxicity.FormatJSON(report)
	default:
		output = toxicity.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "toxicity", err)
	}

	return ExitPass
}
