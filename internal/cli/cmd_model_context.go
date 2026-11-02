package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/joeyvictorino/assay/internal/modelcontext"
)

func init() {
	Register("model-context", "analyze context window utilization across runs", runModelContext)
}

func runModelContext(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-context", stderr)
	runsFile := fs.String("runs", "", "path to JSON file containing run entries (required)")
	format := fs.String("format", "text", "output format: text or json")
	threshold := fs.String("threshold", "0.8", "utilization warning threshold (0.0-1.0)")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runsFile == "" {
		fmt.Fprintln(stderr, "assay model-context: --runs is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-context: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	thresh, err := strconv.ParseFloat(*threshold, 64)
	if err != nil || thresh < 0 || thresh > 1 {
		fmt.Fprintf(stderr, "assay model-context: --threshold must be between 0.0 and 1.0\n")
		return ExitError
	}

	data, err := os.ReadFile(*runsFile)
	if err != nil {
		return fail(stderr, "model-context", err)
	}

	var entries []modelcontext.RunEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fail(stderr, "model-context", fmt.Errorf("parsing runs JSON: %w", err))
	}

	report := modelcontext.Analyze(entries, thresh)

	var output string
	switch *format {
	case "json":
		output = modelcontext.FormatJSON(report) + "\n"
	default:
		output = modelcontext.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-context", err)
	}
	return ExitPass
}
