package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/checkeragree"
)

func init() {
	Register("checker-agree", "measure inter-checker agreement with Cohen's kappa", runCheckerAgree)
}

func runCheckerAgree(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay checker-agree", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay checker-agree: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "checker-agree", err)
	}

	report := checkeragree.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = checkeragree.FormatJSON(report)
	default:
		output = checkeragree.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "checker-agree", err)
	}

	return ExitPass
}
