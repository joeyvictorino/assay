package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/costbreak"
)

func init() {
	Register("cost", "break down API costs from a run", runCost)
}

func runCost(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay cost", stderr)
	runDir := fs.String("path", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	groupBy := fs.String("group-by", "", "group results by: checker, model, or prompt")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" {
		fmt.Fprintln(stderr, "assay cost: --path is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay cost: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*runDir, runJSON))
	if err != nil {
		return fail(stderr, "cost", err)
	}

	report := costbreak.AnalyzeCost(rr)

	switch *format {
	case "json":
		if err := writeJSON("", report, stdout); err != nil {
			return fail(stderr, "cost", err)
		}
	default:
		text := costbreak.FormatText(report, *groupBy)
		if _, err := io.WriteString(stdout, text); err != nil {
			return fail(stderr, "cost", err)
		}
	}
	return ExitPass
}
