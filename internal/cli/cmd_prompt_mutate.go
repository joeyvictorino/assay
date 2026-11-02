package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/promptmutate"
)

func init() {
	Register("prompt-mutate", "test prompt robustness via mutation analysis", runPromptMutate)
}

func runPromptMutate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-mutate", stderr)
	promptFile := fs.String("prompt", "", "file containing the original prompt text (required)")
	findingsFile := fs.String("findings", "", "original findings JSON file (required)")
	mutatedFile := fs.String("mutated-results", "", "JSON object mapping prompt-hash to findings arrays")
	format := fs.String("format", "text", "text | json")
	threshold := fs.Float64("threshold", 0.5, "stability threshold (0.0-1.0)")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay prompt-mutate: unknown --format %q (text | json)\n", *format)
		return ExitError
	}
	if *promptFile == "" {
		fmt.Fprintln(stderr, "assay prompt-mutate: --prompt is required")
		return ExitError
	}
	if *findingsFile == "" {
		fmt.Fprintln(stderr, "assay prompt-mutate: --findings is required")
		return ExitError
	}

	promptData, err := os.ReadFile(*promptFile)
	if err != nil {
		return fail(stderr, "prompt-mutate", err)
	}
	prompt := string(promptData)

	ff, err := readFindings(*findingsFile, nil)
	if err != nil {
		return fail(stderr, "prompt-mutate", err)
	}

	var report promptmutate.Report
	if *mutatedFile != "" {
		mutData, err := os.ReadFile(*mutatedFile)
		if err != nil {
			return fail(stderr, "prompt-mutate", err)
		}
		var mutResults map[string][]model.Finding
		if err := json.Unmarshal(mutData, &mutResults); err != nil {
			return fail(stderr, "prompt-mutate", fmt.Errorf("parse mutated results: %w", err))
		}
		report = promptmutate.AnalyzeStatic(prompt, ff, mutResults, nil, *threshold)
	} else {
		report = promptmutate.Analyze(prompt, ff, nil, nil, *threshold)
	}

	var output string
	switch *format {
	case "json":
		output = promptmutate.FormatJSON(report)
	default:
		output = promptmutate.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "prompt-mutate", err)
	}

	return ExitPass
}
