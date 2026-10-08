package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/reconcile"
)

var sectionOrder = []string{
	"## Scope statement",
	"## Corpus",
	"## Findings",
	"## Overlap",
	"## Evaluation",
	"## Reconciliation",
	"## Policy decisions",
	"## Evidence model",
	"## Limitations",
	"## Runtime",
}

func sampleRun() model.RunReport {
	start := time.Date(2026, 10, 7, 6, 0, 0, 0, time.UTC)
	return model.RunReport{
		RunID:    "r1",
		GitSHA:   "abc123",
		CIRunURL: "https://example.invalid/actions/runs/1",
		Mode:     "full",
		Started:  start,
		Finished: start.Add(90 * time.Second),
		Scope:    model.Decision{Effect: model.EffectAllow, Reason: "SCOPE_OK"},
		Labs:     []string{"synthetic-ops"},
		Models: []model.ModelResult{
			{Ref: model.ModelRef{Provider: "openai", Model: "m-b"}, Findings: map[model.FindingState]int{model.StateValidated: 1, model.StateTheorized: 2}, Usage: model.Usage{CostUSD: 0.5}, P50MS: 100, P95MS: 300, Refusals: 1},
			{Ref: model.ModelRef{Provider: "anthropic", Model: "m-a"}, Findings: map[model.FindingState]int{model.StateValidated: 3}, Usage: model.Usage{CostUSD: 1.25}, P50MS: 200, P95MS: 400},
		},
		Overlap: &model.OverlapMatrix{
			Models:       []string{"m-a", "m-b"},
			Pairwise:     [][]model.PairCell{{{}, {Both: 1, OnlyA: 2, OnlyB: 1, Jaccard: 0.25}}, {{Both: 1, OnlyA: 1, OnlyB: 2, Jaccard: 0.25}, {}}},
			UniquePer:    map[string][]string{"m-a": {"k1", "k2"}, "m-b": {"k3"}},
			Union:        4,
			Intersection: 1,
		},
		Precision:    map[string]model.PRF{"m-a": {TP: 3, FP: 0, FN: 1, Precision: 1, Recall: 0.75, F1: 0.857}},
		PolicyDenies: map[string]int{"DENY_RULE:no-write-in-recon": 2},
		AuditHead:    "deadbeef",
		AuditCount:   42,
		BudgetCapUSD: 15,
		SpentUSD:     1.75,
		Verdict:      "PASS",
	}
}

func TestMarkdownSectionOrder(t *testing.T) {
	cases := []struct {
		name string
		rr   model.RunReport
		fs   []model.Finding
		rec  *reconcile.Report
	}{
		{"full run", sampleRun(), nil, &reconcile.Report{Checked: 3, Mismatches: []reconcile.Mismatch{{ToolCallID: "c1", Reasons: []string{reconcile.ReasonToolNameMismatch}, Claimed: "a", Actual: "b", TranscriptSeqs: []uint64{1}, ControlSeqs: []uint64{2}, Confidence: "high"}}, ReasonCounts: map[string]int{reconcile.ReasonToolNameMismatch: 1}}},
		{"empty run", model.RunReport{}, nil, nil},
		{"findings only", model.RunReport{}, []model.Finding{{ID: "f1", DedupKey: "k", State: model.StateTheorized, Model: model.ModelRef{Model: "x"}}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			md := Markdown(tc.rr, tc.fs, tc.rec)
			last := -1
			for _, s := range sectionOrder {
				idx := strings.Index(md, "\n"+s+"\n")
				if idx < 0 {
					t.Fatalf("missing section %q", s)
				}
				if idx < last {
					t.Fatalf("section %q out of order", s)
				}
				last = idx
			}
			if Markdown(tc.rr, tc.fs, tc.rec) != md {
				t.Fatal("Markdown is not deterministic")
			}
		})
	}
}

func TestMarkdownContent(t *testing.T) {
	rr := sampleRun()
	rec := &reconcile.Report{Checked: 3, Mismatches: []reconcile.Mismatch{{ToolCallID: "c1", Reasons: []string{reconcile.ReasonToolNameMismatch}, Claimed: "http_get", Actual: "http_post", TranscriptSeqs: []uint64{1}, Confidence: "high"}}, ReasonCounts: map[string]int{reconcile.ReasonToolNameMismatch: 1}}
	md := Markdown(rr, nil, rec)
	cases := []struct{ name, want string }{
		{"run id", "# assay run r1"},
		{"ci url", "<https://example.invalid/actions/runs/1>"},
		{"labs", "`synthetic-ops`"},
		{"models sorted", "`m-a`, `m-b`"},
		{"findings row", "| m-a | 3 | 0 | 0 | 0 | 3 |"},
		{"pairwise", "| m-a | m-b | 1 | 2 | 1 | 0.250 |"},
		{"unique per model", "| m-a | 2 |"},
		{"precision row", "| m-a | 3 | 0 | 1 | 1.000 | 0.750 | 0.857 |"},
		{"reconcile summary", "Checked 3 tool call id(s); 1 mismatch(es)."},
		{"mismatch row", "| `c1` | tool-name-mismatch | http_get | http_post | 1 | - | high |"},
		{"policy deny", "| `DENY_RULE:no-write-in-recon` | 2 |"},
		{"audit head", "`deadbeef`"},
		{"audit count", "Audit record count: 42"},
		{"duration", "Duration: 1m30s"},
		{"runtime row", "| m-b | 0.5000 | 100 | 300 | 1 | 0 |"},
		{"verdict", "Verdict: PASS (exit 0)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(md, tc.want) {
				t.Fatalf("missing %q in:\n%s", tc.want, md)
			}
		})
	}
	if strings.Contains(md, NoGroundTruth) {
		t.Fatal("ground-truth sentence printed although precision exists")
	}
}

func TestMarkdownFallbacks(t *testing.T) {
	rr := model.RunReport{RunID: "r2", Mode: "degraded", Reconcile: map[string]int{reconcile.ReasonMissingControl: 4}, Truncated: true}
	md := Markdown(rr, nil, nil)
	for _, want := range []string{
		NoGroundTruth,
		"| missing-control-plane-event | 4 |",
		"No policy denials recorded.",
		"No overlap matrix",
		"Mode: **degraded**",
		"was truncated",
		"No findings were recorded.",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestRowsAndDashboardData(t *testing.T) {
	later := sampleRun()
	later.RunID = "r0-later"
	later.Started = later.Started.Add(48 * time.Hour)
	later.Overlap = nil
	cases := []struct {
		name     string
		runs     []model.RunReport
		wantIDs  []string
		wantJSON string
	}{
		{"empty", nil, nil, "[]\n"},
		{"sorted by date then id", []model.RunReport{later, sampleRun()}, []string{"r1", "r0-later"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := Rows(tc.runs)
			if len(rows) != len(tc.wantIDs) {
				t.Fatalf("rows=%d want %d", len(rows), len(tc.wantIDs))
			}
			for i, id := range tc.wantIDs {
				if rows[i].RunID != id {
					t.Fatalf("row %d = %s want %s", i, rows[i].RunID, id)
				}
			}
			data := DashboardData(tc.runs)
			if tc.wantJSON != "" && string(data) != tc.wantJSON {
				t.Fatalf("data=%q want %q", data, tc.wantJSON)
			}
			if !bytes.Equal(data, DashboardData(tc.runs)) {
				t.Fatal("DashboardData not byte-stable")
			}
			var back []map[string]any
			if err := json.Unmarshal(data, &back); err != nil {
				t.Fatal(err)
			}
			if len(back) != len(rows) {
				t.Fatalf("round trip rows=%d", len(back))
			}
		})
	}
	rows := Rows([]model.RunReport{sampleRun()})
	r := rows[0]
	if r.Date != "2026-10-07" || r.OverlapUnion != 4 || r.OverlapIntersection != 1 || r.Verdict != "PASS" {
		t.Fatalf("row=%+v", r)
	}
	if len(r.Models) != 2 || r.Models[0].Model != "m-a" || r.Models[1].FindingsTheorized != 2 || r.Models[1].Refusals != 1 || r.Models[1].P95MS != 300 {
		t.Fatalf("models=%+v", r.Models)
	}
	for _, key := range []string{`"run_id"`, `"date"`, `"ci_run_url"`, `"findings_validated"`, `"findings_theorized"`, `"refusals"`, `"cost_usd"`, `"p50_ms"`, `"p95_ms"`, `"overlap_union"`, `"overlap_intersection"`, `"verdict"`} {
		if !bytes.Contains(DashboardData(rows2run()), []byte(key)) {
			t.Fatalf("dashboard json missing %s", key)
		}
	}
}

func rows2run() []model.RunReport { return []model.RunReport{sampleRun()} }

func TestMarkdownSelfReflectionTable(t *testing.T) {
	rr := sampleRun()
	rr.Reflection = []model.ReflectionResult{
		{Lab: "lab-a", Model: "m1", Scored: true, Scores: []int{4, 9}, Revisions: 1, Final: 9, Passed: true},
		{Lab: "lab-b", Model: "m1", Scored: false, Note: "the model did not report a score"},
	}
	md := Markdown(rr, nil, nil)
	for _, want := range []string{
		"### Self-reflection",
		"| lab-a | m1 | 4, 9 | 1 | 9 | true |",
		"| lab-b | m1 | - | 0 | - | false | the model did not report a score |",
		"The score gates revision only",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q:\n%s", want, md)
		}
	}
	// the table sits inside the Evaluation section, before Reconciliation
	if strings.Index(md, "### Self-reflection") < strings.Index(md, "## Evaluation") ||
		strings.Index(md, "### Self-reflection") > strings.Index(md, "## Reconciliation") {
		t.Error("self-reflection must sit between Evaluation and Reconciliation")
	}
	// absent when a run has no reflection records
	rr.Reflection = nil
	if strings.Contains(Markdown(rr, nil, nil), "### Self-reflection") {
		t.Error("table printed for a run with no reflection")
	}
}
