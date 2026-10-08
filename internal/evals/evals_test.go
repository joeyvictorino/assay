package evals

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func iptr(i int) *int              { return &i }
func fptr(f float64) *float64      { return &f }
func i64ptr(i int64) *int64        { return &i }
func names(rs []CaseResult) string { return strings.Join(namesOf(rs), ",") }

func namesOf(rs []CaseResult) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Name)
	}
	return out
}

func report() model.RunReport {
	return model.RunReport{
		RunID: "r1",
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Provider: "local", Model: "qwen-3b"},
				Findings: map[model.FindingState]int{model.StateValidated: 3, model.StateTheorized: 5},
				Usage:    model.Usage{CostUSD: 0}, P95MS: 80000, Refusals: 0},
			{Ref: model.ModelRef{Provider: "fake", Model: "fake-model"},
				Findings: map[model.FindingState]int{model.StateTheorized: 6},
				Usage:    model.Usage{CostUSD: 0.0025}, P95MS: 4, Refusals: 1},
		},
		Reconcile: map[string]int{"duplicate-control-tool-call-id": 3, "duplicate-transcript-tool-call-id": 3},
	}
}

func finding(modelID, lab string, state model.FindingState, mismatch bool) model.Finding {
	return model.Finding{Lab: lab, State: state, Model: model.ModelRef{Model: modelID},
		Reconcile: model.ReconcileStatus{Checked: true, Match: !mismatch}}
}

func findings() []model.Finding {
	var fs []model.Finding
	for i := 0; i < 3; i++ {
		fs = append(fs, finding("qwen-3b", "synthetic-ops", model.StateValidated, false))
	}
	for i := 0; i < 4; i++ {
		fs = append(fs, finding("qwen-3b", "synthetic-ops", model.StateTheorized, false))
	}
	fs = append(fs, finding("qwen-3b", "dvwa", model.StateTheorized, false))
	for i := 0; i < 6; i++ {
		fs = append(fs, finding("fake-model", "synthetic-ops", model.StateTheorized, true))
	}
	return fs
}

func TestEvaluate(t *testing.T) {
	cases := []struct {
		name         string
		c            Case
		nilFindings  bool
		wantPassed   bool
		wantAdvisory bool // lands in Verdict.Advisory
		wantReason   string
		wantMiss     []string // check names that failed
	}{
		{name: "min validated met", c: Case{Name: "a", ModelGlob: "qwen*", MinValidated: iptr(3)}, wantPassed: true},
		{name: "min validated missed", c: Case{Name: "a", ModelGlob: "qwen*", MinValidated: iptr(4)}, wantMiss: []string{"min_validated"}},
		{name: "lab filter changes counts", c: Case{Name: "a", ModelGlob: "qwen*", Lab: "synthetic-ops", MaxTheorizedRatio: fptr(0.58)}, wantPassed: true},
		{name: "ratio without lab filter", c: Case{Name: "a", ModelGlob: "qwen*", MaxTheorizedRatio: fptr(0.58)}, wantMiss: []string{"max_theorized_ratio"}},
		{name: "ratio from report counts", c: Case{Name: "a", ModelGlob: "qwen*", MaxTheorizedRatio: fptr(0.625)}, nilFindings: true, wantPassed: true},
		{name: "lab needs findings", c: Case{Name: "a", ModelGlob: "qwen*", Lab: "dvwa", MinValidated: iptr(0)}, nilFindings: true, wantReason: "lab filter needs"},
		{name: "cost summed across models", c: Case{Name: "a", MaxCostUSD: fptr(0.002)}, wantMiss: []string{"max_cost_usd"}},
		{name: "cost within band", c: Case{Name: "a", MaxCostUSD: fptr(0.003)}, wantPassed: true},
		{name: "p95 is the max over models", c: Case{Name: "a", MaxP95MS: i64ptr(79999)}, wantMiss: []string{"max_p95_ms"}},
		{name: "p95 within band", c: Case{Name: "a", ModelGlob: "fake-*", MaxP95MS: i64ptr(4)}, wantPassed: true},
		{name: "mismatches from findings", c: Case{Name: "a", ModelGlob: "fake-*", MaxReconcileMismatches: iptr(6)}, wantPassed: true},
		{name: "mismatches filtered by model", c: Case{Name: "a", ModelGlob: "qwen*", MaxReconcileMismatches: iptr(0)}, wantPassed: true},
		{name: "mismatches from report when no findings", c: Case{Name: "a", MaxReconcileMismatches: iptr(5)}, nilFindings: true, wantMiss: []string{"max_reconcile_mismatches"}},
		{name: "refusals", c: Case{Name: "a", RequireZeroRefusals: true}, wantMiss: []string{"require_zero_refusals"}},
		{name: "refusals scoped by glob", c: Case{Name: "a", ModelGlob: "qwen*", RequireZeroRefusals: true}, wantPassed: true},
		{name: "no model matches", c: Case{Name: "a", ModelGlob: "gpt-*", MinValidated: iptr(0)}, wantReason: "no model in the run matches"},
		{name: "no bands", c: Case{Name: "a", ModelGlob: "*"}, wantReason: "no bands"},
		{name: "advisory miss does not fail", c: Case{Name: "a", Advisory: true, MinValidated: iptr(99)}, wantAdvisory: true, wantMiss: []string{"min_validated"}},
		{name: "advisory pass is a pass", c: Case{Name: "a", Advisory: true, MinValidated: iptr(1)}, wantPassed: true},
		{name: "several misses listed", c: Case{Name: "a", MinValidated: iptr(99), MaxP95MS: i64ptr(1), RequireZeroRefusals: true}, wantMiss: []string{"min_validated", "max_p95_ms", "require_zero_refusals"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := findings()
			if tc.nilFindings {
				fs = nil
			}
			v := Evaluate([]Case{tc.c}, report(), fs)
			if v.RunID != "r1" {
				t.Fatalf("run id = %q", v.RunID)
			}
			var res CaseResult
			switch {
			case tc.wantPassed:
				if len(v.Passed) != 1 || len(v.Failed) != 0 || len(v.Advisory) != 0 || !v.OK() {
					t.Fatalf("verdict = %+v, want one pass", v)
				}
				res = v.Passed[0]
			case tc.wantAdvisory:
				if len(v.Advisory) != 1 || len(v.Failed) != 0 || !v.OK() {
					t.Fatalf("verdict = %+v, want one advisory miss", v)
				}
				res = v.Advisory[0]
			default:
				if len(v.Failed) != 1 || v.OK() {
					t.Fatalf("verdict = %+v, want one failure", v)
				}
				res = v.Failed[0]
			}
			if tc.wantReason != "" && !strings.Contains(res.Reason, tc.wantReason) {
				t.Fatalf("reason = %q, want %q", res.Reason, tc.wantReason)
			}
			var missed []string
			for _, ch := range res.Checks {
				if !ch.Passed {
					missed = append(missed, ch.Name)
				}
			}
			if strings.Join(missed, ",") != strings.Join(tc.wantMiss, ",") {
				t.Fatalf("missed checks = %v, want %v (checks %+v)", missed, tc.wantMiss, res.Checks)
			}
		})
	}
}

func TestWithDefaults(t *testing.T) {
	th := Thresholds{Defaults: Case{MaxCostUSD: fptr(2), MaxP95MS: i64ptr(100), MaxReconcileMismatches: iptr(1), MaxTheorizedRatio: fptr(0.9), MinValidated: iptr(0)}}
	cases := []struct {
		name string
		in   Case
		want Case
	}{
		{"all inherited", Case{Name: "x"}, Case{Name: "x", MaxCostUSD: fptr(2), MaxP95MS: i64ptr(100), MaxReconcileMismatches: iptr(1), MaxTheorizedRatio: fptr(0.9), MinValidated: iptr(0)}},
		{"explicit wins", Case{Name: "x", MaxCostUSD: fptr(0.1), MaxP95MS: i64ptr(5)}, Case{Name: "x", MaxCostUSD: fptr(0.1), MaxP95MS: i64ptr(5), MaxReconcileMismatches: iptr(1), MaxTheorizedRatio: fptr(0.9), MinValidated: iptr(0)}},
		{"explicit zero is kept", Case{Name: "x", MaxCostUSD: fptr(0)}, Case{Name: "x", MaxCostUSD: fptr(0), MaxP95MS: i64ptr(100), MaxReconcileMismatches: iptr(1), MaxTheorizedRatio: fptr(0.9), MinValidated: iptr(0)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := WithDefaults([]Case{tc.in}, th)[0]
			gj, _ := json.Marshal(got)
			wj, _ := json.Marshal(tc.want)
			if string(gj) != string(wj) {
				t.Fatalf("got %s, want %s", gj, wj)
			}
		})
	}
	if out := WithDefaults(nil, th); len(out) != 0 {
		t.Fatalf("nil cases = %v", out)
	}
}

func TestParseCase(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{"minimal", "name: a\nmin_validated: 1\n", ""},
		{"all fields", "name: a\nlab: l\nmodel_glob: 'q*'\nmin_validated: 1\nmax_theorized_ratio: 0.5\nmax_cost_usd: 1\nmax_p95_ms: 10\nmax_reconcile_mismatches: 0\nrequire_zero_refusals: true\nadvisory: true\nnote: n\n", ""},
		{"empty", "\n", "empty document"},
		{"no name", "min_validated: 1\n", "name is required"},
		{"unknown field", "name: a\nmin_validate: 1\n", "yaml"},
		{"bad glob", "name: a\nmodel_glob: '[x'\n", "model_glob"},
		{"negative validated", "name: a\nmin_validated: -1\n", "min_validated"},
		{"ratio above one", "name: a\nmax_theorized_ratio: 1.5\n", "max_theorized_ratio"},
		{"negative cost", "name: a\nmax_cost_usd: -1\n", "max_cost_usd"},
		{"negative p95", "name: a\nmax_p95_ms: -1\n", "max_p95_ms"},
		{"negative mismatches", "name: a\nmax_reconcile_mismatches: -1\n", "max_reconcile_mismatches"},
		{"wrong type", "name: a\nmin_validated: many\n", "yaml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCase([]byte(tc.raw))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !errors.Is(err, ErrInvalidCase) {
				t.Fatalf("err = %v, want %q wrapping ErrInvalidCase", err, tc.wantErr)
			}
		})
	}
}

func TestParseThresholds(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"ok", "defaults:\n  max_cost_usd: 1\n  max_p95_ms: 5\n", false},
		{"empty defaults", "defaults: {}\n", false},
		{"unknown top-level", "default:\n  max_cost_usd: 1\n", true},
		{"bad band", "defaults:\n  max_theorized_ratio: 2\n", true},
		{"empty", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseThresholds([]byte(tc.raw))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	write := func(t *testing.T, dir string, files map[string]string) {
		t.Helper()
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	cases := []struct {
		name      string
		files     map[string]string
		wantNames []string
		wantErr   string
	}{
		{"sorted by file name", map[string]string{"b.yaml": "name: second\nmin_validated: 1\n", "a.yml": "name: first\nmin_validated: 1\n", "notes.txt": "ignored"}, []string{"first", "second"}, ""},
		{"duplicate names", map[string]string{"a.yaml": "name: same\n", "b.yaml": "name: same\n"}, nil, "duplicate case name"},
		{"invalid file named", map[string]string{"a.yaml": "name: ok\n", "bad.yaml": "nme: x\n"}, nil, "bad.yaml"},
		{"no files", map[string]string{"readme.md": "x"}, nil, "no case files"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, tc.files)
			got, err := Load(dir)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var ns []string
			for _, c := range got {
				ns = append(ns, c.Name)
				if c.Source == "" {
					t.Errorf("case %s has no source", c.Name)
				}
			}
			if strings.Join(ns, ",") != strings.Join(tc.wantNames, ",") {
				t.Fatalf("names = %v, want %v", ns, tc.wantNames)
			}
		})
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("Load of a missing dir succeeded")
	}
}

// TestCommittedRunSatisfiesCases ties the shipped cases to the committed
// run: four bands hold and the advisory target is reported but not failed.
func TestCommittedRunSatisfiesCases(t *testing.T) {
	root := filepath.Join("..", "..")
	latest, err := os.ReadFile(filepath.Join(root, "results", "LATEST"))
	if err != nil {
		t.Skip("no results/LATEST:", err)
	}
	runDir := filepath.Join(root, "results", strings.TrimSpace(string(latest)))
	var rr model.RunReport
	readJSON(t, filepath.Join(runDir, "run.json"), &rr)
	var fs []model.Finding
	readJSON(t, filepath.Join(runDir, "findings.redacted.json"), &fs)

	cases, err := Load(filepath.Join(root, "evals", "cases"))
	if err != nil {
		t.Fatal(err)
	}
	th, err := LoadThresholds(filepath.Join(root, "evals", "thresholds.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	v := Evaluate(WithDefaults(cases, th), rr, fs)
	if !v.OK() || len(v.Passed) != 4 || len(v.Advisory) != 1 {
		t.Fatalf("verdict: passed=%s failed=%s advisory=%s\n%s", names(v.Passed), names(v.Failed), names(v.Advisory), Markdown(v))
	}
	if v.Advisory[0].Name != "local-model-precision-target" {
		t.Fatalf("advisory = %s", v.Advisory[0].Name)
	}
	md := Markdown(v)
	for _, want := range []string{"PASS: 4 passed, 0 failed, 1 advisory misses", "### Advisory", "### Passed", rr.RunID} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

func TestMarkdownFailure(t *testing.T) {
	v := Evaluate([]Case{{Name: "needs-more", MinValidated: iptr(99)}}, report(), findings())
	md := Markdown(v)
	for _, want := range []string{"FAIL: 0 passed, 1 failed", "### Failed", "needs-more", "min_validated: MISS >= 99 observed 3"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}
}
