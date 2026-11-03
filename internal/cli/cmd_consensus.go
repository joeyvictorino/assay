package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/consensus"
)

func init() {
	Register("consensus", "compute multi-model consensus on findings", runConsensus)
}

func runConsensus(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay consensus", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	minModels := fs.Int("min-models", 2, "minimum models per group for consensus")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay consensus: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "consensus", err)
	}

	results := consensus.Analyze(ff, *minModels)

	var output string
	switch *format {
	case "json":
		output = consensus.FormatJSON(results)
	default:
		output = consensus.FormatText(results)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "consensus", err)
	}

	return ExitPass
}
