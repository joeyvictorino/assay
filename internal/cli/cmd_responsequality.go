package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/responsequality"
)

func init() {
	Register("response-quality", "score response quality across completeness, specificity, actionability, clarity", runResponseQuality)
}

func runResponseQuality(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay response-quality", stderr)
	findings := fs.String("findings", "", "path to findings JSON file (required)")
	format := fs.String("format", "text", "output format: text or json")
	threshold := fs.Float64("threshold", 0.5, "quality flag threshold (0.0-1.0)")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *findings == "" {
		fmt.Fprintln(stderr, "assay response-quality: --findings is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay response-quality: unknown --format %q (text | json)\n", *format)
		return ExitError
	}
	if *threshold < 0 || *threshold > 1 {
		fmt.Fprintln(stderr, "assay response-quality: --threshold must be between 0.0 and 1.0")
		return ExitError
	}

	b, err := os.ReadFile(*findings)
	if err != nil {
		return fail(stderr, "response-quality", fmt.Errorf("read %s: %w", *findings, err))
	}
	var ff []model.Finding
	if err := json.Unmarshal(b, &ff); err != nil {
		return fail(stderr, "response-quality", fmt.Errorf("parse %s: %w", *findings, err))
	}

	report := responsequality.Analyze(ff, *threshold)

	var output string
	switch *format {
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fail(stderr, "response-quality", err)
		}
		output = string(data) + "\n"
	default:
		output = responsequality.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "response-quality", err)
	}
	return ExitPass
}
