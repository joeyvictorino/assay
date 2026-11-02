package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/promptanalyze"
)

func init() {
	Register("prompt-analyze", "analyze prompt effectiveness across findings", runPromptAnalyze)
}

func runPromptAnalyze(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay prompt-analyze", stderr)
	runDir := fs.String("path", "", "path to run directory containing findings (required)")
	format := fs.String("format", "text", "output format: text or json")
	sortBy := fs.String("sort", "findings", "sort by: findings or score")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" {
		fmt.Fprintln(stderr, "assay prompt-analyze: --path is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay prompt-analyze: unknown --format %q (text | json)\n", *format)
		return ExitError
	}
	switch *sortBy {
	case "findings", "score":
	default:
		fmt.Fprintf(stderr, "assay prompt-analyze: unknown --sort %q (findings | score)\n", *sortBy)
		return ExitError
	}

	p := filepath.Join(*runDir, findingsJSON)
	if !fileExists(p) {
		fmt.Fprintln(stderr, "assay prompt-analyze: no findings file found in run directory")
		return ExitError
	}
	findings, err := readFindings(p, nil)
	if err != nil {
		return fail(stderr, "prompt-analyze", err)
	}

	pa := promptanalyze.Analyze(findings)

	if *sortBy == "score" {
		promptanalyze.SortByScore(pa.Prompts)
		if len(pa.Prompts) > 0 {
			pa.MostEffective = pa.Prompts[0].PromptHash
			pa.LeastEffective = pa.Prompts[len(pa.Prompts)-1].PromptHash
		}
	}

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(pa, "", "  ")
		if err != nil {
			return fail(stderr, "prompt-analyze", err)
		}
		output = string(b) + "\n"
	default:
		output = promptanalyze.FormatText(pa)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "prompt-analyze", err)
	}
	return ExitPass
}
