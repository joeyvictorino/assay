package cli

import (
	"fmt"
	"io"

	"github.com/joeyvictorino/assay/internal/continuous"
	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("continuous", "compare a run against the promoted baseline: diff", runContinuous)
}

func runContinuous(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		continuousUsage(stderr)
		return ExitError
	}
	switch args[0] {
	case "diff":
		return continuousDiff(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		continuousUsage(stdout)
		return ExitPass
	}
	fmt.Fprintf(stderr, "assay continuous: unknown action %q\n", args[0])
	continuousUsage(stderr)
	return ExitError
}

func continuousUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  assay continuous diff --baseline FILE --current FILE")
	fmt.Fprintln(w, "                        [--baseline-findings F --current-findings F]")
	fmt.Fprintln(w, "                        [--fail-on SEVERITY] [--out FILE]")
	fmt.Fprintln(w, "  prints the issue body as Markdown; writes the delta as JSON to --out")
	fmt.Fprintln(w, "  exit 0 no drift at or above --fail-on, 1 drift, 2 error")
}

// continuousDiff implements `assay continuous diff`. FILE arguments are
// run.json documents; the findings files are JSON arrays of model.Finding.
// Without findings files the delta covers cost and latency only.
func continuousDiff(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay continuous diff", stderr)
	basePath := fs.String("baseline", "", "baseline run.json (required)")
	curPath := fs.String("current", "", "current run.json (required)")
	baseFindings := fs.String("baseline-findings", "", "baseline findings JSON array")
	curFindings := fs.String("current-findings", "", "current findings JSON array")
	failOn := fs.String("fail-on", string(model.SevHigh), "minimum severity of a new or regressed finding that fails the run (info|low|medium|high|critical)")
	out := fs.String("out", "", "write the delta as JSON to this file")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *basePath == "" || *curPath == "" {
		fmt.Fprintln(stderr, "assay continuous diff: --baseline and --current are required")
		return ExitError
	}
	sev := model.Severity(*failOn)
	if sev.Rank() == 0 && sev != model.SevInfo {
		fmt.Fprintf(stderr, "assay continuous diff: unknown --fail-on %q\n", *failOn)
		return ExitError
	}
	base, err := readRunReport(*basePath)
	if err != nil {
		return fail(stderr, "continuous diff", err)
	}
	cur, err := readRunReport(*curPath)
	if err != nil {
		return fail(stderr, "continuous diff", err)
	}
	var bf, cf []model.Finding
	if *baseFindings != "" {
		if bf, err = readFindings(*baseFindings, nil); err != nil {
			return fail(stderr, "continuous diff", err)
		}
	}
	if *curFindings != "" {
		if cf, err = readFindings(*curFindings, nil); err != nil {
			return fail(stderr, "continuous diff", err)
		}
	}
	d := continuous.Diff(base, cur, bf, cf)
	if *out != "" {
		if err := writeJSON(*out, d, stdout); err != nil {
			return fail(stderr, "continuous diff", err)
		}
	}
	if _, err := io.WriteString(stdout, continuous.Markdown(d)); err != nil {
		return fail(stderr, "continuous diff", err)
	}
	if continuous.Verdict(d, sev, continuous.SeverityIndex(bf, cf)) != 0 {
		fmt.Fprintf(stderr, "assay continuous diff: drift at or above %s (new=%d regressed=%d)\n", sev, len(d.New), len(d.Regressed))
		return ExitBlocked
	}
	return ExitPass
}
