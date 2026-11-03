package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/drift"
)

func init() {
	Register("drift", "detect model output drift between two finding sets", runDrift)
}

func runDrift(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay drift", stderr)
	findingsA := fs.String("findings-a", "", "findings JSON file for run A (required)")
	findingsB := fs.String("findings-b", "", "findings JSON file for run B (required)")
	format := fs.String("format", "text", "text | json")
	threshold := fs.Float64("threshold", 0.3, "overall drift threshold for flagging")
	out := fs.String("out", "", "write output to this file (\"-\" for stdout, default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *findingsA == "" || *findingsB == "" {
		fmt.Fprintln(stderr, "assay drift: --findings-a and --findings-b are required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay drift: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	fa, err := readFindings(*findingsA, nil)
	if err != nil {
		return fail(stderr, "drift", err)
	}
	fb, err := readFindings(*findingsB, nil)
	if err != nil {
		return fail(stderr, "drift", err)
	}

	runAName := labelFromPath(*findingsA)
	runBName := labelFromPath(*findingsB)

	report := drift.Analyze(fa, fb, runAName, runBName, *threshold)

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fail(stderr, "drift", err)
		}
		output = string(b) + "\n"
	default:
		output = drift.FormatText(report)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "drift", err)
	}

	if report.Summary.FlaggedCount > 0 {
		fmt.Fprintf(stderr, "assay drift: %d finding(s) flagged above threshold %.2f\n",
			report.Summary.FlaggedCount, *threshold)
		return ExitBlocked
	}
	return ExitPass
}

func labelFromPath(p string) string {
	if p == "-" || p == "" {
		return "stdin"
	}
	return p
}

