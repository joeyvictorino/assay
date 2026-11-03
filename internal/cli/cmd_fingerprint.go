package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/fingerprint"
)

func init() {
	Register("model-fingerprint", "model response style fingerprinting and comparison", runFingerprint)
}

func runFingerprint(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-fingerprint", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-fingerprint: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	ff, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "model-fingerprint", err)
	}

	r := fingerprint.Analyze(ff)

	var output string
	switch *format {
	case "json":
		output = fingerprint.FormatJSON(r)
	default:
		output = fingerprint.FormatText(r)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-fingerprint", err)
	}

	return ExitPass
}
