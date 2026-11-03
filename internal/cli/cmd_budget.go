package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/budget"
)

func init() {
	Register("budget", "show token budget usage for a run", runBudget)
}

func runBudget(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay budget", stderr)
	runDir := fs.String("path", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	limit := fs.Int("limit", 0, "limit rows shown per section (0 = all)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" {
		fmt.Fprintln(stderr, "assay budget: --path is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay budget: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*runDir, runJSON))
	if err != nil {
		return fail(stderr, "budget", err)
	}

	report := budget.Analyze(rr)

	switch *format {
	case "json":
		if err := writeJSON("", report, stdout); err != nil {
			return fail(stderr, "budget", err)
		}
	default:
		text := budget.FormatText(report, *limit)
		if _, err := io.WriteString(stdout, text); err != nil {
			return fail(stderr, "budget", err)
		}
	}
	return ExitPass
}
