package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/reconcile"
	"github.com/joeyvictorino/assay/internal/report"
)

// Results-directory file names (the lead's `run` command writes them).
const (
	runJSON        = "run.json"
	findingsJSON   = "findings.redacted.json"
	reconcileJSON  = "reconcile.json"
	reportMarkdown = "report.md"
	dashboardIndex = "docs/dashboard/data/index.json"
)

func init() {
	Register("report", "render a results directory as Markdown or refresh the dashboard data", runReport)
}

// runReport implements:
//
//	assay report --run DIR [--format md|dashboard] [--runs-root results/] [--out FILE]
//
// md (default) reads DIR/run.json, DIR/findings.redacted.json and
// DIR/reconcile.json (the last two optional) and writes DIR/report.md.
// dashboard reads every <runs-root>/*/run.json and writes
// docs/dashboard/data/index.json. --out overrides the output path; "-"
// writes to stdout.
func runReport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay report", stderr)
	runDir := fs.String("run", "", "results directory for one run (required)")
	format := fs.String("format", "md", "md | dashboard")
	runsRoot := fs.String("runs-root", "results", "directory holding one sub-directory per run (dashboard format)")
	out := fs.String("out", "", "output path (default DIR/report.md or docs/dashboard/data/index.json; \"-\" for stdout)")
	if err := fs.Parse(args); err != nil {
		return ExitError
	}
	if *runDir == "" {
		fmt.Fprintln(stderr, "assay report: --run is required")
		fs.Usage()
		return ExitError
	}
	switch *format {
	case "md":
		return reportMarkdownCmd(*runDir, *out, stdout, stderr)
	case "dashboard":
		return reportDashboard(*runsRoot, *out, stdout, stderr)
	}
	fmt.Fprintf(stderr, "assay report: unknown --format %q (md | dashboard)\n", *format)
	return ExitError
}

func reportMarkdownCmd(runDir, out string, stdout, stderr io.Writer) int {
	rr, err := readRunReport(filepath.Join(runDir, runJSON))
	if err != nil {
		return fail(stderr, "report", err)
	}
	var findings []model.Finding
	if p := filepath.Join(runDir, findingsJSON); fileExists(p) {
		findings, err = readFindings(p, nil)
		if err != nil {
			return fail(stderr, "report", err)
		}
	}
	var rec *reconcile.Report
	if p := filepath.Join(runDir, reconcileJSON); fileExists(p) {
		b, err := os.ReadFile(p)
		if err != nil {
			return fail(stderr, "report", err)
		}
		var r reconcile.Report
		if err := json.Unmarshal(b, &r); err != nil {
			return fail(stderr, "report", fmt.Errorf("%s: %w", p, err))
		}
		rec = &r
	}
	md := report.Markdown(rr, findings, rec)
	if out == "" {
		out = filepath.Join(runDir, reportMarkdown)
	}
	if err := writeText(out, md, stdout); err != nil {
		return fail(stderr, "report", err)
	}
	if out != "-" {
		fmt.Fprintf(stdout, "report: wrote %s (%d bytes)\n", out, len(md))
	}
	return ExitPass
}

func reportDashboard(runsRoot, out string, stdout, stderr io.Writer) int {
	entries, err := os.ReadDir(runsRoot)
	if err != nil {
		return fail(stderr, "report", err)
	}
	var runs []model.RunReport
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		p := filepath.Join(runsRoot, name, runJSON)
		if !fileExists(p) {
			continue
		}
		rr, err := readRunReport(p)
		if err != nil {
			return fail(stderr, "report", err)
		}
		runs = append(runs, rr)
	}
	data := report.DashboardData(runs)
	if out == "" {
		out = dashboardIndex
	}
	if out == "-" {
		_, err := stdout.Write(data)
		if err != nil {
			return fail(stderr, "report", err)
		}
		return ExitPass
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fail(stderr, "report", err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return fail(stderr, "report", err)
	}
	fmt.Fprintf(stdout, "report: dashboard rows=%d out=%s\n", len(runs), out)
	return ExitPass
}

func readRunReport(path string) (model.RunReport, error) {
	var rr model.RunReport
	b, err := os.ReadFile(path)
	if err != nil {
		return rr, err
	}
	if err := json.Unmarshal(b, &rr); err != nil {
		return rr, fmt.Errorf("%s: %w", path, err)
	}
	if rr.RunID == "" {
		return rr, errors.New(path + ": run_id is empty")
	}
	return rr, nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
