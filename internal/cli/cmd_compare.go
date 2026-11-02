package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/continuous"
	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("compare", "diff two assay runs: new, fixed and regressed findings", runCompare)
}

func runCompare(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay compare", stderr)
	baseDir := fs.String("baseline", "", "results directory for the baseline run (required)")
	curDir := fs.String("current", "", "results directory for the current run (required)")
	format := fs.String("format", "text", "text | json")
	failOn := fs.String("fail-on", "", "exit 1 when any new or regressed finding is at or above this severity (info|low|medium|high|critical)")
	out := fs.String("out", "", "write output to this file (\"-\" for stdout, default stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *baseDir == "" || *curDir == "" {
		fmt.Fprintln(stderr, "assay compare: --baseline and --current are required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "text", "json":
	default:
		fmt.Fprintf(stderr, "assay compare: unknown --format %q (text | json)\n", *format)
		return ExitError
	}

	base, err := readRunReport(filepath.Join(*baseDir, runJSON))
	if err != nil {
		return fail(stderr, "compare", err)
	}
	cur, err := readRunReport(filepath.Join(*curDir, runJSON))
	if err != nil {
		return fail(stderr, "compare", err)
	}

	var bf, cf []model.Finding
	if p := filepath.Join(*baseDir, findingsJSON); fileExists(p) {
		if bf, err = readFindings(p, nil); err != nil {
			return fail(stderr, "compare", err)
		}
	}
	if p := filepath.Join(*curDir, findingsJSON); fileExists(p) {
		if cf, err = readFindings(p, nil); err != nil {
			return fail(stderr, "compare", err)
		}
	}

	d := continuous.Diff(base, cur, bf, cf)

	var output string
	switch *format {
	case "json":
		b, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return fail(stderr, "compare", err)
		}
		output = string(b) + "\n"
	default:
		output = continuous.Markdown(d)
	}

	dest := *out
	if dest == "" {
		dest = "-"
	}
	if err := writeText(dest, output, stdout); err != nil {
		return fail(stderr, "compare", err)
	}

	if *failOn != "" {
		sev := model.Severity(*failOn)
		if sev.Rank() == 0 && sev != model.SevInfo {
			fmt.Fprintf(stderr, "assay compare: unknown --fail-on %q\n", *failOn)
			return ExitError
		}
		if continuous.Verdict(d, sev, continuous.SeverityIndex(bf, cf)) != 0 {
			fmt.Fprintf(stderr, "assay compare: drift at or above %s (new=%d regressed=%d)\n", sev, len(d.New), len(d.Regressed))
			return ExitBlocked
		}
	}
	return ExitPass
}
