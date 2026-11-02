package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/ablation"
	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("ablation", "analyze prompt component impact via ablation testing", runAblation)
}

func runAblation(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay ablation", stderr)
	fullPath := fs.String("full", "", "full-prompt findings JSON file (required)")
	ablatedPath := fs.String("ablated", "", "ablated findings JSON file: {\"component\": [...findings]} (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay ablation: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *fullPath == "" {
		fmt.Fprintln(stderr, "assay ablation: --full is required")
		return ExitError
	}
	if *ablatedPath == "" {
		fmt.Fprintln(stderr, "assay ablation: --ablated is required")
		return ExitError
	}

	fullFindings, err := readFindings(*fullPath, nil)
	if err != nil {
		return fail(stderr, "ablation", err)
	}

	ablatedData, err := os.ReadFile(*ablatedPath)
	if err != nil {
		return fail(stderr, "ablation", err)
	}
	var raw map[string][]model.Finding
	if err := json.Unmarshal(ablatedData, &raw); err != nil {
		return fail(stderr, "ablation", fmt.Errorf("ablated file: %w", err))
	}

	ablatedMap := make(map[ablation.Component][]model.Finding, len(raw))
	for k, v := range raw {
		ablatedMap[ablation.Component(k)] = v
	}

	report := ablation.Analyze(fullFindings, ablatedMap)

	var output string
	switch *format {
	case "json":
		output = ablation.FormatJSON(report)
	default:
		output = ablation.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "ablation", err)
	}

	return ExitPass
}
