package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/grounding"
	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("grounding", "factual grounding verification for findings", runGrounding)
}

func runGrounding(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay grounding", stderr)
	findings := fs.String("findings", "", "path to findings JSON file (required)")
	format := fs.String("format", "text", "output format: text or json")
	threshold := fs.Float64("threshold", 0.5, "grounding threshold (0.0-1.0); findings below are flagged")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *findings == "" {
		fmt.Fprintln(stderr, "assay grounding: --findings is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay grounding: unknown --format %q (text | json)\n", *format)
		return ExitError
	}
	if *threshold < 0 || *threshold > 1 {
		fmt.Fprintln(stderr, "assay grounding: --threshold must be between 0.0 and 1.0")
		return ExitError
	}

	b, err := os.ReadFile(*findings)
	if err != nil {
		return fail(stderr, "grounding", fmt.Errorf("read %s: %w", *findings, err))
	}
	var ff []model.Finding
	if err := json.Unmarshal(b, &ff); err != nil {
		return fail(stderr, "grounding", fmt.Errorf("parse %s: %w", *findings, err))
	}

	report := grounding.Analyze(ff, *threshold)

	var output string
	switch *format {
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fail(stderr, "grounding", err)
		}
		output = string(data) + "\n"
	default:
		output = grounding.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "grounding", err)
	}
	return ExitPass
}
