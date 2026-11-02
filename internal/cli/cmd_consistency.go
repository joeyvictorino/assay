package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/joeyvictorino/assay/internal/consistency"
)

func init() {
	Register("consistency", "cross-run finding consistency scoring", runConsistency)
}

func runConsistency(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay consistency", stderr)
	paths := fs.String("paths", "", "comma-separated paths to findings JSON files (required, min 2)")
	threshold := fs.Float64("threshold", 0.8, "minimum presence rate to consider a finding stable")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *paths == "" {
		fmt.Fprintln(stderr, "assay consistency: --paths is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay consistency: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	runPaths := splitPaths(*paths)
	if len(runPaths) < 2 {
		fmt.Fprintln(stderr, "assay consistency: --paths must contain at least 2 files")
		return ExitError
	}

	report, err := consistency.AnalyzeConsistency(runPaths, *threshold)
	if err != nil {
		return fail(stderr, "consistency", err)
	}

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fail(stderr, "consistency", err)
		}
		output = string(b) + "\n"
	default:
		output = consistency.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "consistency", err)
	}
	return ExitPass
}

func splitPaths(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
