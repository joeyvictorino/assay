package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/proportionality"
)

func init() {
	Register("proportionality", "analyze response length proportionality relative to query complexity", runProportionality)
}

func runProportionality(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay proportionality", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay proportionality: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "proportionality", err)
	}

	report := proportionality.Analyze(data)
	output := proportionality.FormatText(report)

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "proportionality", err)
	}

	return ExitPass
}
