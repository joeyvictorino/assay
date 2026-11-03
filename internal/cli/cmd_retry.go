package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/retry"
)

func init() {
	Register("retry", "analyze failed run and produce retry plan with backoff", runRetry)
}

func runRetry(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay retry", stderr)
	runDir := fs.String("path", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	maxRetries := fs.Int("max-retries", 3, "maximum retry attempts for backoff schedule")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" {
		fmt.Fprintln(stderr, "assay retry: --path is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay retry: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*runDir, runJSON))
	if err != nil {
		return fail(stderr, "retry", err)
	}

	plan := retry.Analyze(rr, *maxRetries)

	switch *format {
	case "json":
		if err := writeJSON("", plan, stdout); err != nil {
			return fail(stderr, "retry", err)
		}
	default:
		text := retry.FormatText(plan)
		if _, err := io.WriteString(stdout, text); err != nil {
			return fail(stderr, "retry", err)
		}
	}
	return ExitPass
}
