package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/tokeneff"
)

func init() {
	Register("token-efficiency", "score model token usage efficiency across a run", runTokenEff)
}

func runTokenEff(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay token-efficiency", stderr)
	runDir := fs.String("path", "", "path to run directory containing run.json (required)")
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" {
		fmt.Fprintln(stderr, "assay token-efficiency: --path is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay token-efficiency: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	rr, err := readRunReport(filepath.Join(*runDir, runJSON))
	if err != nil {
		return fail(stderr, "token-efficiency", err)
	}

	report := tokeneff.Analyze(rr)

	switch *format {
	case "json":
		if err := writeJSON("", report, stdout); err != nil {
			return fail(stderr, "token-efficiency", err)
		}
	default:
		text := tokeneff.FormatText(report)
		if _, err := io.WriteString(stdout, text); err != nil {
			return fail(stderr, "token-efficiency", err)
		}
	}
	return ExitPass
}
