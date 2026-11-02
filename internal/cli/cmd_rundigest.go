package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/rundigest"
)

func init() {
	Register("run-digest", "condensed one-line and one-paragraph run summary for CI/Slack", runRunDigest)
}

func runRunDigest(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay run-digest", stderr)
	run := fs.String("run", "", "path to run report JSON (required)")
	format := fs.String("format", "text", "output format: text, json, or oneline")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *run == "" {
		fmt.Fprintln(stderr, "assay run-digest: --run is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json", "oneline":
	default:
		fmt.Fprintf(stderr, "assay run-digest: unknown --format %q (text | json | oneline)\n", *format)
		return ExitError
	}

	rr, err := readRunReport(*run)
	if err != nil {
		return fail(stderr, "run-digest", err)
	}

	digest := rundigest.Summarize(rr)

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(digest, "", "  ")
		if err != nil {
			return fail(stderr, "run-digest", err)
		}
		output = string(b) + "\n"
	case "oneline":
		output = digest.OneLine + "\n"
	default:
		output = rundigest.FormatText(digest)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "run-digest", err)
	}
	return ExitPass
}
