package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/modelsafety"
)

func init() {
	Register("model-safety", "evaluate model safety boundaries and refusal behavior", runModelSafety)
}

func runModelSafety(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-safety", stderr)
	dir := fs.String("run", "", "path to run directory (required)")
	format := fs.String("format", "text", "output format: text or json")
	threshold := fs.Float64("threshold", 0.8, "pass/fail threshold (0.0-1.0)")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *dir == "" {
		fmt.Fprintln(stderr, "assay model-safety: --run is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay model-safety: unknown --format %q (text | json)\n", *format)
		return ExitError
	}
	if *threshold < 0 || *threshold > 1 {
		fmt.Fprintf(stderr, "assay model-safety: --threshold must be between 0.0 and 1.0\n")
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*dir, runJSON))
	if err != nil {
		return fail(stderr, "model-safety", err)
	}

	var findings []model.Finding
	if p := filepath.Join(*dir, findingsJSON); fileExists(p) {
		b, err := os.ReadFile(p)
		if err != nil {
			return fail(stderr, "model-safety", fmt.Errorf("read %s: %w", p, err))
		}
		if err := json.Unmarshal(b, &findings); err != nil {
			return fail(stderr, "model-safety", fmt.Errorf("parse %s: %w", p, err))
		}
	}

	result := modelsafety.Analyze(rr, findings, *threshold)

	var output string
	switch *format {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fail(stderr, "model-safety", err)
		}
		output = string(data) + "\n"
	default:
		output = modelsafety.FormatText(result)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-safety", err)
	}
	return ExitPass
}
