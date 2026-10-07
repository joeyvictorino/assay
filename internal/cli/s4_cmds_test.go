package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/reconcile"
)

func runS4(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Main(args, &out, &errb)
	return code, out.String(), errb.String()
}

func writeJSONFile(t *testing.T, dir, name string, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func auditExportFile(t *testing.T, dir string, recs []model.AuditRecord) string {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, r := range recs {
		if err := enc.Encode(r); err != nil {
			t.Fatal(err)
		}
	}
	buf.WriteString("\n") // blank line must be tolerated
	p := filepath.Join(dir, "audit.export.jsonl")
	if err := os.WriteFile(p, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReconcileCommand(t *testing.T) {
	claims := func(cs ...model.ClaimedToolCall) map[string]any { return map[string]any{"claimed_tool_calls": cs} }
	cases := []struct {
		name         string
		records      []model.AuditRecord
		findings     []model.Finding
		wantCode     int
		wantMismatch int
		wantStates   []model.FindingState
	}{
		{
			name: "clean",
			records: []model.AuditRecord{
				{Seq: 1, Kind: "model_call", Agent: "probe", Meta: claims(model.ClaimedToolCall{ID: "c1", Name: "http_get", ArgsDigest: "d"})},
				{Seq: 2, Kind: "tool_call", Agent: "probe", ToolCallID: "c1", Tool: "http_get", ArgsDigest: "d"},
			},
			findings:   []model.Finding{{ID: "f1", State: model.StateValidated, ToolCallIDs: []string{"c1"}}},
			wantCode:   ExitPass,
			wantStates: []model.FindingState{model.StateValidated},
		},
		{
			name: "mismatch downgrades and exits 1",
			records: []model.AuditRecord{
				{Seq: 1, Kind: "model_call", Agent: "probe", Meta: claims(model.ClaimedToolCall{ID: "c1", Name: "http_get", ArgsDigest: "d"})},
				{Seq: 2, Kind: "tool_call", Agent: "probe", ToolCallID: "c1", Tool: "http_post", ArgsDigest: "d"},
			},
			findings:     []model.Finding{{ID: "f1", State: model.StateValidated, ToolCallIDs: []string{"c1"}}, {ID: "f2", State: model.StateValidated}},
			wantCode:     ExitBlocked,
			wantMismatch: 1,
			wantStates:   []model.FindingState{model.StateTheorized, model.StateValidated},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			exp := auditExportFile(t, dir, tc.records)
			fp := writeJSONFile(t, dir, "findings.json", tc.findings)
			outDir := filepath.Join(dir, "out")
			code, stdout, stderr := runS4(t, "reconcile", "--audit-export", exp, "--findings", fp, "--out", outDir)
			if code != tc.wantCode {
				t.Fatalf("code=%d out=%q err=%q", code, stdout, stderr)
			}
			b, err := os.ReadFile(filepath.Join(outDir, "reconcile.json"))
			if err != nil {
				t.Fatal(err)
			}
			var rep reconcile.Report
			if err := json.Unmarshal(b, &rep); err != nil {
				t.Fatal(err)
			}
			if len(rep.Mismatches) != tc.wantMismatch {
				t.Fatalf("mismatches=%d", len(rep.Mismatches))
			}
			fb, err := os.ReadFile(filepath.Join(outDir, "findings.reconciled.json"))
			if err != nil {
				t.Fatal(err)
			}
			var fs []model.Finding
			if err := json.Unmarshal(fb, &fs); err != nil {
				t.Fatal(err)
			}
			for i, want := range tc.wantStates {
				if fs[i].State != want || !fs[i].Reconcile.Checked {
					t.Fatalf("finding %d state=%s reconcile=%+v", i, fs[i].State, fs[i].Reconcile)
				}
			}
		})
	}
}

func TestReconcileCommandErrors(t *testing.T) {
	dir := t.TempDir()
	fp := writeJSONFile(t, dir, "findings.json", []model.Finding{})
	bad := filepath.Join(dir, "bad.jsonl")
	if err := os.WriteFile(bad, []byte("{\"kind\":\"model_call\"}\nnot json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.jsonl")
	if err := os.WriteFile(empty, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		args []string
	}{
		{"missing flags", []string{"reconcile", "--findings", fp}},
		{"bad line", []string{"reconcile", "--audit-export", bad, "--findings", fp, "--out", dir}},
		{"empty export", []string{"reconcile", "--audit-export", empty, "--findings", fp, "--out", dir}},
		{"missing export", []string{"reconcile", "--audit-export", filepath.Join(dir, "nope"), "--findings", fp, "--out", dir}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code, _, _ := runS4(t, tc.args...); code != ExitError {
				t.Fatalf("code=%d", code)
			}
		})
	}
}

func sampleRunReport(id string, started time.Time) model.RunReport {
	return model.RunReport{
		RunID: id, Mode: "full", Started: started, Finished: started.Add(time.Minute),
		Labs:    []string{"synthetic-ops"},
		Models:  []model.ModelResult{{Ref: model.ModelRef{Provider: "fake", Model: "fake-model"}, Findings: map[model.FindingState]int{model.StateValidated: 2}, Usage: model.Usage{CostUSD: 0.01}, P50MS: 5, P95MS: 9}},
		Overlap: &model.OverlapMatrix{Models: []string{"fake-model"}, Union: 2, Intersection: 2},
		Verdict: "PASS", SpentUSD: 0.01, BudgetCapUSD: 15, AuditHead: "ab", AuditCount: 3,
	}
}

func TestReportMarkdownCommand(t *testing.T) {
	dir := t.TempDir()
	runDir := filepath.Join(dir, "20261007-r1")
	writeJSONFile(t, runDir, "run.json", sampleRunReport("r1", time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)))
	writeJSONFile(t, runDir, "findings.redacted.json", []model.Finding{{ID: "f1", DedupKey: "k1", State: model.StateValidated, Model: model.ModelRef{Model: "fake-model"}}})
	writeJSONFile(t, runDir, "reconcile.json", reconcile.Report{Checked: 2, Mismatches: []reconcile.Mismatch{}, ReasonCounts: map[string]int{}})

	code, stdout, stderr := runS4(t, "report", "--run", runDir)
	if code != ExitPass {
		t.Fatalf("code=%d out=%q err=%q", code, stdout, stderr)
	}
	md, err := os.ReadFile(filepath.Join(runDir, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"# assay run r1", "## Scope statement", "Checked 2 tool call id(s)", "## Runtime"} {
		if !strings.Contains(string(md), want) {
			t.Fatalf("missing %q", want)
		}
	}
	// stdout output
	code, stdout, _ = runS4(t, "report", "--run", runDir, "--out", "-")
	if code != ExitPass || !strings.HasPrefix(stdout, "# assay run r1") {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
	// errors
	cases := []struct {
		name string
		args []string
	}{
		{"no run", []string{"report"}},
		{"bad format", []string{"report", "--run", runDir, "--format", "pdf"}},
		{"missing run.json", []string{"report", "--run", dir}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code, _, _ := runS4(t, tc.args...); code != ExitError {
				t.Fatalf("code=%d", code)
			}
		})
	}
}

func TestReportDashboardCommand(t *testing.T) {
	root := t.TempDir()
	writeJSONFile(t, filepath.Join(root, "20261001-a"), "run.json", sampleRunReport("a", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)))
	writeJSONFile(t, filepath.Join(root, "20261007-b"), "run.json", sampleRunReport("b", time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)))
	if err := os.WriteFile(filepath.Join(root, "LATEST"), []byte("20261007-b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "no-run-json"), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(root, "docs", "dashboard", "data", "index.json")
	code, stdout, stderr := runS4(t, "report", "--run", filepath.Join(root, "20261007-b"), "--format", "dashboard", "--runs-root", root, "--out", out)
	if code != ExitPass {
		t.Fatalf("code=%d out=%q err=%q", code, stdout, stderr)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0]["run_id"] != "a" || rows[1]["run_id"] != "b" {
		t.Fatalf("rows=%v", rows)
	}
	if code, _, _ := runS4(t, "report", "--run", root, "--format", "dashboard", "--runs-root", filepath.Join(root, "missing")); code != ExitError {
		t.Fatalf("missing root code=%d", code)
	}
}

func TestContinuousDiffCommand(t *testing.T) {
	dir := t.TempDir()
	base := writeJSONFile(t, dir, "baseline.json", sampleRunReport("base", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)))
	cur := writeJSONFile(t, dir, "current.json", sampleRunReport("cur", time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)))
	bf := writeJSONFile(t, dir, "bf.json", []model.Finding{{ID: "f1", DedupKey: "k1", State: model.StateValidated, Severity: model.SevHigh}})
	cfHigh := writeJSONFile(t, dir, "cf-high.json", []model.Finding{{ID: "f1", DedupKey: "k1", State: model.StateValidated, Severity: model.SevHigh}, {ID: "f2", DedupKey: "k2", State: model.StateTheorized, Severity: model.SevHigh}})
	cfLow := writeJSONFile(t, dir, "cf-low.json", []model.Finding{{ID: "f1", DedupKey: "k1", State: model.StateValidated, Severity: model.SevHigh}, {ID: "f2", DedupKey: "k2", State: model.StateTheorized, Severity: model.SevLow}})
	cases := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
	}{
		{"no findings no drift", []string{"continuous", "diff", "--baseline", base, "--current", cur}, ExitPass, "| New | 0 |"},
		{"new high fails", []string{"continuous", "diff", "--baseline", base, "--current", cur, "--baseline-findings", bf, "--current-findings", cfHigh}, ExitBlocked, "- `k2`"},
		{"new low passes at high", []string{"continuous", "diff", "--baseline", base, "--current", cur, "--baseline-findings", bf, "--current-findings", cfLow}, ExitPass, "| New | 1 |"},
		{"new low fails at low", []string{"continuous", "diff", "--baseline", base, "--current", cur, "--baseline-findings", bf, "--current-findings", cfLow, "--fail-on", "low"}, ExitBlocked, "| New | 1 |"},
		{"missing flags", []string{"continuous", "diff", "--baseline", base}, ExitError, ""},
		{"bad severity", []string{"continuous", "diff", "--baseline", base, "--current", cur, "--fail-on", "urgent"}, ExitError, ""},
		{"unknown action", []string{"continuous", "frob"}, ExitError, ""},
		{"no action", []string{"continuous"}, ExitError, ""},
		{"help", []string{"continuous", "help"}, ExitPass, "usage:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runS4(t, tc.args...)
			if code != tc.wantCode {
				t.Fatalf("code=%d out=%q err=%q", code, stdout, stderr)
			}
			if tc.wantOut != "" && !strings.Contains(stdout, tc.wantOut) {
				t.Fatalf("missing %q in %q", tc.wantOut, stdout)
			}
		})
	}
	outPath := filepath.Join(dir, "delta.json")
	if code, _, _ := runS4(t, "continuous", "diff", "--baseline", base, "--current", cur, "--baseline-findings", bf, "--current-findings", cfHigh, "--out", outPath); code != ExitBlocked {
		t.Fatalf("code=%d", code)
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		New []string `json:"new"`
	}
	if err := json.Unmarshal(b, &d); err != nil || len(d.New) != 1 || d.New[0] != "k2" {
		t.Fatalf("delta=%s err=%v", b, err)
	}
}
