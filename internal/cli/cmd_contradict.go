package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/contradict"
)

func init() {
	Register("contradict", "detect self-contradictions, retractions, and inconsistencies", runContradict)
}

func runContradict(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay contradict", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay contradict: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay contradict: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "contradict", err)
	}

	report := contradict.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = contradict.FormatJSON(report)
	default:
		output = contradict.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "contradict", err)
	}

	return ExitPass
}
