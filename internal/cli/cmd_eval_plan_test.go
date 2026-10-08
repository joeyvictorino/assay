package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Main(args, &out, &errb)
	return code, out.String(), errb.String()
}

func writeFile(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func evalFixture(t *testing.T, withFindings bool) (runDir, casesDir, thresholds string) {
	t.Helper()
	dir := t.TempDir()
	runDir = filepath.Join(dir, "run")
	rr := model.RunReport{RunID: "r1", Models: []model.ModelResult{
		{Ref: model.ModelRef{Model: "m-one"}, Findings: map[model.FindingState]int{model.StateValidated: 2, model.StateTheorized: 1},
			Usage: model.Usage{CostUSD: 0.5}, P95MS: 100},
	}}
	writeJSONFile(t, runDir, "run.json", rr)
	if withFindings {
		fs := []model.Finding{
			{Lab: "lab-a", State: model.StateValidated, Model: model.ModelRef{Model: "m-one"}},
			{Lab: "lab-a", State: model.StateValidated, Model: model.ModelRef{Model: "m-one"}},
			{Lab: "lab-b", State: model.StateTheorized, Model: model.ModelRef{Model: "m-one"}},
		}
		writeJSONFile(t, runDir, "findings.redacted.json", fs)
	}
	casesDir = filepath.Join(dir, "cases")
	writeFile(t, filepath.Join(casesDir, "pass.yaml"), "name: pass\nmodel_glob: 'm-*'\nmin_validated: 2\n")
	writeFile(t, filepath.Join(casesDir, "advisory.yaml"), "name: target\nadvisory: true\nmin_validated: 10\n")
	thresholds = writeFile(t, filepath.Join(dir, "thresholds.yaml"), "defaults:\n  max_cost_usd: 1\n  max_p95_ms: 1000\n")
	return runDir, casesDir, thresholds
}

func TestEvalCommand(t *testing.T) {
	cases := []struct {
		name         string
		withFindings bool
		extraCase    string // body of an extra case file, "" for none
		args         func(runDir, casesDir, thresholds string) []string
		wantCode     int
		wantOut      []string
		wantErr      []string
	}{
		{
			name: "pass with findings", withFindings: true,
			args:     func(r, c, th string) []string { return []string{"eval", "--run", r, "--cases", c, "--thresholds", th} },
			wantCode: ExitPass,
			wantOut:  []string{"PASS: 1 passed, 0 failed, 1 advisory misses", "target"},
		},
		{
			name:     "pass without findings falls back to report counts",
			args:     func(r, c, th string) []string { return []string{"eval", "--run", r, "--cases", c, "--thresholds", th} },
			wantCode: ExitPass,
			wantErr:  []string{"findings.redacted.json not found"},
		},
		{
			name: "lab case without findings fails", extraCase: "name: lab\nlab: lab-a\nmin_validated: 1\n",
			args:     func(r, c, th string) []string { return []string{"eval", "--run", r, "--cases", c, "--thresholds", th} },
			wantCode: ExitBlocked,
			wantOut:  []string{"lab filter needs"},
			wantErr:  []string{"1 case(s) regressed"},
		},
		{
			name: "lab case with findings passes", withFindings: true, extraCase: "name: lab\nlab: lab-a\nmin_validated: 2\n",
			args:     func(r, c, th string) []string { return []string{"eval", "--run", r, "--cases", c, "--thresholds", th} },
			wantCode: ExitPass,
		},
		{
			name: "regression exits 1", withFindings: true, extraCase: "name: tight\nmax_cost_usd: 0.1\n",
			args:     func(r, c, th string) []string { return []string{"eval", "--run", r, "--cases", c, "--thresholds", th} },
			wantCode: ExitBlocked,
			wantOut:  []string{"FAIL: 1 passed, 1 failed", "max_cost_usd: MISS"},
		},
		{
			name: "default band regression exits 1", withFindings: true, extraCase: "name: inherits\nrequire_zero_refusals: true\n",
			args: func(r, c, th string) []string {
				tight := writeFile(t, filepath.Join(filepath.Dir(th), "tight.yaml"), "defaults:\n  max_p95_ms: 10\n")
				return []string{"eval", "--run", r, "--cases", c, "--thresholds", tight}
			},
			wantCode: ExitBlocked,
			wantOut:  []string{"max_p95_ms: MISS <= 10 observed 100"},
		},
		{
			name:     "missing run flag",
			args:     func(r, c, th string) []string { return []string{"eval", "--cases", c} },
			wantCode: ExitError,
			wantErr:  []string{"--run is required"},
		},
		{
			name: "missing run dir",
			args: func(r, c, th string) []string {
				return []string{"eval", "--run", r + "-nope", "--cases", c, "--thresholds", th}
			},
			wantCode: ExitError,
		},
		{
			name: "missing cases dir",
			args: func(r, c, th string) []string {
				return []string{"eval", "--run", r, "--cases", c + "-nope", "--thresholds", th}
			},
			wantCode: ExitError,
		},
		{
			name: "missing thresholds",
			args: func(r, c, th string) []string {
				return []string{"eval", "--run", r, "--cases", c, "--thresholds", th + ".nope"}
			},
			wantCode: ExitError,
		},
		{
			name: "invalid case file", extraCase: "name: bad\nmin_validated: -3\n",
			args:     func(r, c, th string) []string { return []string{"eval", "--run", r, "--cases", c, "--thresholds", th} },
			wantCode: ExitError,
			wantErr:  []string{"min_validated"},
		},
		{
			name:     "help",
			args:     func(r, c, th string) []string { return []string{"eval", "--help"} },
			wantCode: ExitPass,
			wantOut:  []string{"usage:"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runDir, casesDir, th := evalFixture(t, tc.withFindings)
			if tc.extraCase != "" {
				writeFile(t, filepath.Join(casesDir, "zz-extra.yaml"), tc.extraCase)
			}
			code, out, errOut := runCLI(t, tc.args(runDir, casesDir, th)...)
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tc.wantCode, out, errOut)
			}
			for _, w := range tc.wantOut {
				if !strings.Contains(out, w) {
					t.Errorf("stdout lacks %q:\n%s", w, out)
				}
			}
			for _, w := range tc.wantErr {
				if !strings.Contains(errOut, w) {
					t.Errorf("stderr lacks %q:\n%s", w, errOut)
				}
			}
		})
	}
}

func TestEvalCommandWritesJSON(t *testing.T) {
	runDir, casesDir, th := evalFixture(t, true)
	out := filepath.Join(t.TempDir(), "verdict.json")
	code, _, errOut := runCLI(t, "eval", "--run", runDir, "--cases", casesDir, "--thresholds", th, "--out", out)
	if code != ExitPass {
		t.Fatalf("exit = %d: %s", code, errOut)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{`"run_id": "r1"`, `"passed"`, `"advisory"`} {
		if !strings.Contains(string(b), w) {
			t.Errorf("verdict json lacks %q:\n%s", w, b)
		}
	}
}

func TestEvalCommandOnCommittedRun(t *testing.T) {
	latest, err := os.ReadFile(filepath.Join("..", "..", "results", "LATEST"))
	if err != nil {
		t.Skip("no results/LATEST:", err)
	}
	runDir := filepath.Join("..", "..", "results", strings.TrimSpace(string(latest)))
	code, out, errOut := runCLI(t, "eval", "--run", runDir,
		"--cases", filepath.Join("..", "..", "evals", "cases"),
		"--thresholds", filepath.Join("..", "..", "evals", "thresholds.yaml"))
	if code != ExitPass {
		t.Fatalf("exit = %d\n%s\n%s", code, out, errOut)
	}
	// The counts change as runs are committed; assert the run and the verdict.
	id := strings.TrimSpace(string(latest))
	if !strings.Contains(out, "Eval verdict for run "+id) || !strings.Contains(out, "PASS:") || strings.Contains(out, "FAIL:") {
		t.Fatalf("stdout:\n%s", out)
	}
}

func TestPlanCommand(t *testing.T) {
	valid := "tasks:\n  - id: a\n    kind: recon\n    agent: recon\n    lab: l\n  - id: b\n    kind: probe\n    agent: probe\n    lab: l\n    depends_on: [a]\n  - id: c\n    kind: probe\n    agent: probe\n    lab: l\n    depends_on: [a]\n  - id: d\n    kind: validate\n    agent: v\n    lab: l\n    depends_on: [b, c]\n"
	cyclic := "tasks:\n  - id: a\n    kind: recon\n    agent: r\n    lab: l\n    depends_on: [b]\n  - id: b\n    kind: recon\n    agent: r\n    lab: l\n    depends_on: [a]\n"
	cases := []struct {
		name     string
		plan     string // file body; "" means pass a missing path
		args     func(file string) []string
		wantCode int
		wantOut  []string
		wantErr  []string
	}{
		{"validate ok", valid, func(f string) []string { return []string{"plan", "validate", "--plan", f} }, ExitPass, []string{"plan OK: 4 tasks in 3 waves"}, nil},
		{"waves text", valid, func(f string) []string { return []string{"plan", "waves", "--plan", f} }, ExitPass, []string{"wave 0: a\nwave 1: b, c\nwave 2: d\n"}, nil},
		{"waves json", valid, func(f string) []string { return []string{"plan", "waves", "--plan", f, "--json"} }, ExitPass, []string{`"id": "d"`, `"wave": 2`}, nil},
		{"validate cycle", cyclic, func(f string) []string { return []string{"plan", "validate", "--plan", f} }, ExitBlocked, nil, []string{"cycle: a -> b -> a"}},
		{"waves cycle", cyclic, func(f string) []string { return []string{"plan", "waves", "--plan", f} }, ExitBlocked, nil, []string{"cycle"}},
		{"unknown dependency", "tasks:\n  - id: a\n    kind: recon\n    agent: r\n    lab: l\n    depends_on: [zz]\n", func(f string) []string { return []string{"plan", "validate", "--plan", f} }, ExitBlocked, nil, []string{`unknown task "zz"`}},
		{"unknown field", "tasks:\n  - id: a\n    bogus: 1\n", func(f string) []string { return []string{"plan", "validate", "--plan", f} }, ExitBlocked, nil, []string{"unknown field"}},
		{"missing file", "", func(f string) []string { return []string{"plan", "validate", "--plan", f} }, ExitError, nil, nil},
		{"missing flag", valid, func(f string) []string { return []string{"plan", "validate"} }, ExitError, nil, []string{"--plan is required"}},
		{"no action", valid, func(f string) []string { return []string{"plan"} }, ExitError, nil, []string{"usage:"}},
		{"unknown action", valid, func(f string) []string { return []string{"plan", "run", "--plan", f} }, ExitError, nil, []string{"unknown action"}},
		{"help", valid, func(f string) []string { return []string{"plan", "help"} }, ExitPass, []string{"usage:"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "plan.yaml")
			if tc.plan != "" {
				writeFile(t, file, tc.plan)
			}
			code, out, errOut := runCLI(t, tc.args(file)...)
			if code != tc.wantCode {
				t.Fatalf("exit = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tc.wantCode, out, errOut)
			}
			for _, w := range tc.wantOut {
				if !strings.Contains(out, w) {
					t.Errorf("stdout lacks %q:\n%s", w, out)
				}
			}
			for _, w := range tc.wantErr {
				if !strings.Contains(errOut, w) {
					t.Errorf("stderr lacks %q:\n%s", w, errOut)
				}
			}
		})
	}
}

func TestPlanCommandExample(t *testing.T) {
	code, out, errOut := runCLI(t, "plan", "waves", "--plan", filepath.Join("..", "..", "evals", "plans", "example.yaml"))
	if code != ExitPass {
		t.Fatalf("exit = %d: %s", code, errOut)
	}
	want := "wave 0: recon\nwave 1: probe-auth, probe-headers\nwave 2: validate\nwave 3: remediate, score\n"
	if out != want {
		t.Fatalf("waves:\n%s\nwant:\n%s", out, want)
	}
}
