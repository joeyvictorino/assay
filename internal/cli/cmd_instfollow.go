package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/instfollow"
)

func init() {
	Register("instruction-follow", "analyze instruction following accuracy across findings", runInstFollow)
}

func runInstFollow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay instruction-follow", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay instruction-follow: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "instruction-follow", err)
	}

	report := instfollow.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = instfollow.FormatJSON(report)
	default:
		output = instfollow.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "instruction-follow", err)
	}

	return ExitPass
}
