package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/crosslingual"
	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("cross-lingual", "score finding consistency across prompt languages", runCrossLingual)
}

func runCrossLingual(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay cross-lingual", stderr)
	input := fs.String("input", "", "language findings JSON file: {\"lang\": [...findings]} (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay cross-lingual: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *input == "" {
		fmt.Fprintln(stderr, "assay cross-lingual: --input is required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		return fail(stderr, "cross-lingual", err)
	}
	var byLang map[string][]model.Finding
	if err := json.Unmarshal(data, &byLang); err != nil {
		return fail(stderr, "cross-lingual", fmt.Errorf("input file: %w", err))
	}

	report := crosslingual.Analyze(byLang)

	var output string
	switch *format {
	case "json":
		output = crosslingual.FormatJSON(report)
	default:
		output = crosslingual.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "cross-lingual", err)
	}

	return ExitPass
}
