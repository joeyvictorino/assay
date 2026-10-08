package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/joeyvictorino/assay/internal/evals"
	"github.com/joeyvictorino/assay/internal/model"
)

func init() {
	Register("eval", "check a results directory against the eval cases' regression bands", runEval)
}

func evalUsage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  assay eval --run DIR [--cases evals/cases] [--thresholds evals/thresholds.yaml] [--out FILE]")
	fmt.Fprintln(w, "  DIR holds run.json and, optionally, findings.redacted.json")
	fmt.Fprintln(w, "  prints the verdict as Markdown; writes it as JSON to --out")
	fmt.Fprintln(w, "  exit 0 every case within its band, 1 a case regressed, 2 error")
}

// runEval implements `assay eval`. Advisory cases are reported but never
// change the exit code.
func runEval(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			evalUsage(stdout)
			return ExitPass
		}
	}
	fs := newFlagSet("assay eval", stderr)
	runDir := fs.String("run", "", "results directory holding run.json (required)")
	casesDir := fs.String("cases", filepath.Join("evals", "cases"), "directory of eval case YAML files")
	thresholds := fs.String("thresholds", filepath.Join("evals", "thresholds.yaml"), "default bands YAML file")
	out := fs.String("out", "", "write the verdict as JSON to this file")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" {
		fmt.Fprintln(stderr, "assay eval: --run is required")
		evalUsage(stderr)
		return ExitError
	}
	rr, err := readRunReport(filepath.Join(*runDir, "run.json"))
	if err != nil {
		return fail(stderr, "eval", err)
	}
	var findings []model.Finding
	findingsPath := filepath.Join(*runDir, "findings.redacted.json")
	if _, statErr := os.Stat(findingsPath); statErr == nil {
		if findings, err = readFindings(findingsPath, nil); err != nil {
			return fail(stderr, "eval", err)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fail(stderr, "eval", statErr)
	} else {
		fmt.Fprintf(stderr, "assay eval: %s not found; using per-model counts from run.json\n", findingsPath)
	}
	cases, err := evals.Load(*casesDir)
	if err != nil {
		return fail(stderr, "eval", err)
	}
	th, err := evals.LoadThresholds(*thresholds)
	if err != nil {
		return fail(stderr, "eval", err)
	}
	v := evals.Evaluate(evals.WithDefaults(cases, th), rr, findings)
	if *out != "" {
		if err := writeJSON(*out, v, stdout); err != nil {
			return fail(stderr, "eval", err)
		}
	}
	if _, err := io.WriteString(stdout, evals.Markdown(v)); err != nil {
		return fail(stderr, "eval", err)
	}
	if !v.OK() {
		fmt.Fprintf(stderr, "assay eval: %d case(s) regressed\n", len(v.Failed))
		return ExitBlocked
	}
	return ExitPass
}
