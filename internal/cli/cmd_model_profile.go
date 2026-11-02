package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/modelprofile"
)

func init() {
	Register("model-profile", "generate behavioral profile summarizing model strengths and weaknesses", runModelProfile)
}

func runModelProfile(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay model-profile", stderr)
	input := fs.String("input", "", "JSON file with evaluation results (required)")
	model := fs.String("model", "unknown", "model name for the report")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	if *input == "" {
		fmt.Fprintln(stderr, "assay model-profile: --input is required")
		return ExitError
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		return fail(stderr, "model-profile", err)
	}

	var results []modelprofile.EvalResult
	if err := json.Unmarshal(data, &results); err != nil {
		return fail(stderr, "model-profile", fmt.Errorf("parse input: %w", err))
	}

	report := modelprofile.Analyze(*model, results)

	var output string
	switch *format {
	case "json":
		output = modelprofile.FormatJSON(report)
	default:
		output = modelprofile.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "model-profile", err)
	}

	return ExitPass
}
