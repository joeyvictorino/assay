package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/modelhealth"
)

func init() {
	Register("model-health", "per-model health assessment from a run report", runModelHealth)
}

func runModelHealth(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-health", stderr)
	run := fs.String("run", "", "path to run report JSON (required)")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *run == "" {
		fmt.Fprintln(stderr, "assay model-health: --run is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-health: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	rr, err := readRunReport(*run)
	if err != nil {
		return fail(stderr, "model-health", err)
	}

	report := modelhealth.Analyze(rr)

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fail(stderr, "model-health", err)
		}
		output = string(b) + "\n"
	default:
		output = modelhealth.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-health", err)
	}
	return ExitPass
}
