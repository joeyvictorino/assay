package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/empathy"
)

func init() {
	Register("empathy", "score response empathy, compassion, and emotional support", runEmpathy)
}

func runEmpathy(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("empathy", stderr)
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
			return fail(stderr, "empathy", err)
		}
		defer f.Close()
		r = f
	} else {
		r = os.Stdin
	}

	findings, err := readFindings(*input, r)
	if err != nil {
		return fail(stderr, "empathy", err)
	}

	report := empathy.Analyze(findings)
	var text string
	switch *format {
	case "json":
		text = empathy.FormatJSON(report)
	default:
		text = empathy.FormatText(report)
	}

	if err := writeText(*output, text, stdout); err != nil {
		return fail(stderr, "empathy", err)
	}

	if report.MeanScore < 40 {
		fmt.Fprintf(stderr, "empathy: mean score %.1f below threshold\n", report.MeanScore)
		return ExitError
	}
	return ExitPass
}
