package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/emotiondetect"
)

func init() {
	Register("emotion-detect", "detect emotional tone and manipulation in responses", runEmotionDetect)
}

func runEmotionDetect(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay emotion-detect", stderr)
	findings := fs.String("findings", "", "findings JSON file (required)")
	format := fs.String("format", "text", "text | json")
	out := fs.String("out", "", "write output to file (default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay emotion-detect: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	if *findings == "" {
		fmt.Fprintln(stderr, "assay emotion-detect: --findings is required")
		return ExitError
	}

	data, err := readFindings(*findings, nil)
	if err != nil {
		return fail(stderr, "emotion-detect", err)
	}

	report := emotiondetect.Analyze(data)

	var output string
	switch *format {
	case "json":
		output = emotiondetect.FormatJSON(report)
	default:
		output = emotiondetect.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "emotion-detect", err)
	}

	return ExitPass
}
