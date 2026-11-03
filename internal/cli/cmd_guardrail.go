package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/guardrail"
	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("guardrail", "analyze guardrail effectiveness across security categories", runGuardrail)
}

func runGuardrail(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay guardrail", stderr)
	inputDir := fs.String("input", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *inputDir == "" {
		fmt.Fprintln(stderr, "assay guardrail: --input is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay guardrail: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*inputDir, runJSON))
	if err != nil {
		return fail(stderr, "guardrail", err)
	}

	var findings []model.Finding
	if p := filepath.Join(*inputDir, findingsJSON); fileExists(p) {
		if findings, err = readFindings(p, nil); err != nil {
			return fail(stderr, "guardrail", err)
		}
	}

	report := guardrail.Analyze(rr, findings)

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fail(stderr, "guardrail", err)
		}
		output = string(b) + "\n"
	default:
		output = guardrail.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "guardrail", err)
	}
	return ExitPass
}
