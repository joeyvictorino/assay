package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/groundedness"
)

func init() {
	Register("groundedness", "score factual grounding of model responses against evidence", runGroundedness)
}

func runGroundedness(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay groundedness", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay groundedness: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "groundedness", err)
	}

	report := groundedness.Analyze(data)
	output := groundedness.FormatText(report)

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "groundedness", err)
	}

	return ExitPass
}
