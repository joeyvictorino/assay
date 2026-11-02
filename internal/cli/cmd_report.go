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
	"github.com/joeyvictorino/assay/internal/version"
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
//	assay report --run DIR [--format md|sarif|dashboard] [--runs-root results/] [--out FILE]
//
// md (default) reads DIR/run.json, DIR/findings.redacted.json and
// DIR/reconcile.json (the last two optional) and writes DIR/report.md.
// sarif reads DIR/findings.redacted.json and writes SARIF v2.1.0 JSON
// for upload to GitHub Code Scanning (via github/codeql-action/upload-sarif).
// dashboard reads every <runs-root>/*/run.json and writes
// docs/dashboard/data/index.json. --out overrides the output path; "-"
// writes to stdout.
func runReport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("assay report", stderr)
	runDir := fs.String("run", "", "results directory for one run (required)")
	format := fs.String("format", "md", "md | html | sarif | csv | summary | trend | dashboard | dashboard-html | consolidate | consolidate-html")
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
	case "html":
		return reportHTMLCmd(*runDir, *out, stdout, stderr)
	case "sarif":
		return reportSARIFCmd(*runDir, *out, stdout, stderr)
	case "csv":
		return reportCSVCmd(*runDir, *out, stdout, stderr)
	case "summary":
		return reportSummaryCmd(*runDir, *out, stdout, stderr)
	case "trend":
		return reportTrendCmd(*runsRoot, *out, stdout, stderr)
	case "dashboard":
		return reportDashboard(*runsRoot, *out, stdout, stderr)
	case "dashboard-html":
		return reportDashboardHTML(*runsRoot, *out, stdout, stderr)
	case "consolidate":
		return reportConsolidateCmd(*runsRoot, *out, "text", stdout, stderr)
	case "consolidate-html":
		return reportConsolidateCmd(*runsRoot, *out, "html", stdout, stderr)
	}
	fmt.Fprintf(stderr, "assay report: unknown --format %q (md | html | sarif | csv | summary | trend | dashboard | dashboard-html | consolidate | consolidate-html)\n", *format)
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

func reportHTMLCmd(runDir, out string, stdout, stderr io.Writer) int {
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
	h := report.HTML(rr, findings, rec)
	if out == "" {
		out = filepath.Join(runDir, "report.html")
	}
	if err := writeText(out, h, stdout); err != nil {
		return fail(stderr, "report", err)
	}
	if out != "-" {
		fmt.Fprintf(stdout, "report: wrote HTML to %s (%d bytes)\n", out, len(h))
	}
	return ExitPass
}

func reportSARIFCmd(runDir, out string, stdout, stderr io.Writer) int {
	var findings []model.Finding
	p := filepath.Join(runDir, findingsJSON)
	if fileExists(p) {
		var err error
		findings, err = readFindings(p, nil)
		if err != nil {
			return fail(stderr, "report", err)
		}
	}
	sarif, err := report.FindingsToSARIF(findings, version.Version)
	if err != nil {
		return fail(stderr, "report", err)
	}
	if out == "" {
		out = filepath.Join(runDir, "findings.sarif.json")
	}
	if err := writeText(out, sarif, stdout); err != nil {
		return fail(stderr, "report", err)
	}
	if out != "-" {
		fmt.Fprintf(stdout, "report: wrote SARIF to %s (%d bytes)\n", out, len(sarif))
	}
	return ExitPass
}

func reportCSVCmd(runDir, out string, stdout, stderr io.Writer) int {
	var findings []model.Finding
	p := filepath.Join(runDir, findingsJSON)
	if fileExists(p) {
		var err error
		findings, err = readFindings(p, nil)
		if err != nil {
			return fail(stderr, "report", err)
		}
	}
	csv := report.FindingsCSV(findings)
	if out == "" {
		out = filepath.Join(runDir, "findings.csv")
	}
	if err := writeText(out, csv, stdout); err != nil {
		return fail(stderr, "report", err)
	}
	if out != "-" {
		fmt.Fprintf(stdout, "report: wrote CSV to %s (%d findings)\n", out, len(findings))
	}
	return ExitPass
}

func reportSummaryCmd(runDir, out string, stdout, stderr io.Writer) int {
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
	s := report.BuildSummary(rr, findings, rec)
	js, err := report.SummaryJSON(s)
	if err != nil {
		return fail(stderr, "report", err)
	}
	if out == "" {
		out = filepath.Join(runDir, "summary.json")
	}
	if err := writeText(out, js, stdout); err != nil {
		return fail(stderr, "report", err)
	}
	if out != "-" {
		fmt.Fprintf(stdout, "report: wrote summary to %s\n", out)
	}
	return ExitPass
}

func reportTrendCmd(runsRoot, out string, stdout, stderr io.Writer) int {
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
	tr := report.BuildTrend(runs)
	js, err := report.TrendJSON(tr)
	if err != nil {
		return fail(stderr, "report", err)
	}
	if out == "" {
		out = "trend.json"
	}
	if err := writeText(out, js, stdout); err != nil {
		return fail(stderr, "report", err)
	}
	if out != "-" {
		fmt.Fprintf(stdout, "report: trend runs=%d direction=%s out=%s\n", tr.RunCount, tr.Direction, out)
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

func reportDashboardHTML(runsRoot, out string, stdout, stderr io.Writer) int {
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
	html := report.DashboardHTML(runs)
	if out == "" {
		out = "dashboard.html"
	}
	if err := writeText(out, html, stdout); err != nil {
		return fail(stderr, "report", err)
	}
	if out != "-" {
		fmt.Fprintf(stdout, "report: dashboard-html rows=%d out=%s\n", len(runs), out)
	}
	return ExitPass
}

func reportConsolidateCmd(runsRoot, out, outputFmt string, stdout, stderr io.Writer) int {
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
	if len(runs) == 0 {
		fmt.Fprintln(stderr, "assay report: no runs found under "+runsRoot)
		return ExitError
	}
	cr := report.Consolidate(runs)
	var text string
	switch outputFmt {
	case "html":
		text = report.FormatConsolidateHTML(cr)
	default:
		text = report.FormatConsolidateText(cr)
	}
	if out == "" {
		if outputFmt == "html" {
			out = "consolidate.html"
		} else {
			out = "-"
		}
	}
	if err := writeText(out, text, stdout); err != nil {
		return fail(stderr, "report", err)
	}
	if out != "-" {
		fmt.Fprintf(stdout, "report: consolidate runs=%d trend=%s out=%s\n", cr.TotalRuns, cr.Trend, out)
	}
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
