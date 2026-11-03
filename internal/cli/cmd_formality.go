package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/formality"
)

func init() {
	Register("formality", "score response formality, professionalism, and precision", runFormality)
}

func runFormality(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("formality", stderr)
	input := fs.String("input", "", "findings JSON file (default: stdin)")
	format := fs.String("format", "text", "output format (text|json)")
	output := fs.String("output", "", "output file (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}

	var r *os.File
	if *input != "" {
		f, err := os.Open(*input)
		if err != nil {
			return fail(stderr, "formality", err)
		}
		defer f.Close()
		r = f
	} else {
		r = os.Stdin
	}

	findings, err := readFindings(*input, r)
	if err != nil {
		return fail(stderr, "formality", err)
	}

	report := formality.Analyze(findings)
	var text string
	switch *format {
	case "json":
		text = formality.FormatJSON(report)
	default:
		text = formality.FormatText(report)
	}

	if err := writeText(*output, text, stdout); err != nil {
		return fail(stderr, "formality", err)
	}

	if report.MeanScore < 40 {
		fmt.Fprintf(stderr, "formality: mean score %.1f below threshold\n", report.MeanScore)
		return ExitError
	}
	return ExitPass
}
