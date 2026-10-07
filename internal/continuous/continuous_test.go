package continuous

import (
	"reflect"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func f(key string, state model.FindingState, sev model.Severity) model.Finding {
	return model.Finding{ID: "f-" + key, DedupKey: key, State: state, Severity: sev}
}

func TestDiff(t *testing.T) {
	base := model.RunReport{RunID: "b", SpentUSD: 1.0, Models: []model.ModelResult{
		{Ref: model.ModelRef{Model: "m1"}, P95MS: 400},
		{Ref: model.ModelRef{Model: "gone"}, P95MS: 100},
	}}
	cur := model.RunReport{RunID: "c", SpentUSD: 1.5, Models: []model.ModelResult{
		{Ref: model.ModelRef{Model: "m1"}, P95MS: 250},
		{Ref: model.ModelRef{Model: "added"}, P95MS: 900},
	}}
	cases := []struct {
		name          string
		baseF, curF   []model.Finding
		wantNew       []string
		wantFixed     []string
		wantRegressed []string
	}{
		{
			name:    "identical",
			baseF:   []model.Finding{f("k1", model.StateValidated, model.SevHigh)},
			curF:    []model.Finding{f("k1", model.StateValidated, model.SevHigh)},
			wantNew: []string{}, wantFixed: []string{}, wantRegressed: []string{},
		},
		{
			name:    "new and fixed",
			baseF:   []model.Finding{f("old", model.StateTheorized, model.SevLow)},
			curF:    []model.Finding{f("zeta", model.StateValidated, model.SevHigh), f("alpha", model.StateTheorized, model.SevLow)},
			wantNew: []string{"alpha", "zeta"}, wantFixed: []string{"old"}, wantRegressed: []string{},
		},
		{
			name:    "refuted now counts as fixed, refuted before and active now is regressed",
			baseF:   []model.Finding{f("a", model.StateValidated, model.SevHigh), f("b", model.StateRefuted, model.SevHigh)},
			curF:    []model.Finding{f("a", model.StateRefuted, model.SevHigh), f("b", model.StateValidated, model.SevHigh)},
			wantNew: []string{}, wantFixed: []string{"a"}, wantRegressed: []string{"b"},
		},
		{
			name:    "severity increase is a regression, decrease is not",
			baseF:   []model.Finding{f("up", model.StateTheorized, model.SevLow), f("down", model.StateTheorized, model.SevHigh)},
			curF:    []model.Finding{f("up", model.StateTheorized, model.SevCritical), f("down", model.StateTheorized, model.SevLow)},
			wantNew: []string{}, wantFixed: []string{}, wantRegressed: []string{"up"},
		},
		{
			name:    "refuted and declined are never new",
			baseF:   nil,
			curF:    []model.Finding{f("r", model.StateRefuted, model.SevHigh), f("d", model.StateDeclined, model.SevHigh)},
			wantNew: []string{}, wantFixed: []string{}, wantRegressed: []string{},
		},
		{
			name:    "duplicate keys collapse",
			baseF:   nil,
			curF:    []model.Finding{f("k", model.StateTheorized, model.SevLow), f("k", model.StateValidated, model.SevHigh)},
			wantNew: []string{"k"}, wantFixed: []string{}, wantRegressed: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Diff(base, cur, tc.baseF, tc.curF)
			if !reflect.DeepEqual(d.New, tc.wantNew) || !reflect.DeepEqual(d.Fixed, tc.wantFixed) || !reflect.DeepEqual(d.Regressed, tc.wantRegressed) {
				t.Fatalf("new=%v fixed=%v regressed=%v", d.New, d.Fixed, d.Regressed)
			}
			if d.BaselineRunID != "b" || d.CurrentRunID != "c" {
				t.Fatalf("ids %s %s", d.BaselineRunID, d.CurrentRunID)
			}
			if d.CostDeltaUSD != 0.5 {
				t.Fatalf("cost delta %v", d.CostDeltaUSD)
			}
			if !reflect.DeepEqual(d.P95DeltaMS, map[string]int64{"m1": -150}) {
				t.Fatalf("p95 %v", d.P95DeltaMS)
			}
		})
	}
}

func TestDiffKeyDerivedWhenMissing(t *testing.T) {
	a := model.Finding{Lab: "l", Class: model.ClassSQLi, Location: model.Location{Method: "GET", PathTemplate: "/x/1", Param: "q"}, State: model.StateTheorized}
	b := a
	b.Location.PathTemplate = "/X/2"
	d := Diff(model.RunReport{}, model.RunReport{}, []model.Finding{a}, []model.Finding{b})
	if !d.Empty() {
		t.Fatalf("equivalent locations should collapse: %+v", d)
	}
}

func TestVerdict(t *testing.T) {
	sev := map[string]model.Severity{"hi": model.SevHigh, "lo": model.SevLow, "crit": model.SevCritical}
	sevOf := func(k string) model.Severity { return sev[k] }
	cases := []struct {
		name   string
		d      Delta
		failOn model.Severity
		sevOf  func(string) model.Severity
		want   int
	}{
		{"empty delta", Delta{}, model.SevHigh, sevOf, 0},
		{"new below threshold", Delta{New: []string{"lo"}}, model.SevHigh, sevOf, 0},
		{"new at threshold", Delta{New: []string{"hi"}}, model.SevHigh, sevOf, 1},
		{"regressed above threshold", Delta{Regressed: []string{"crit"}}, model.SevHigh, sevOf, 1},
		{"fixed never fails", Delta{Fixed: []string{"crit"}}, model.SevInfo, sevOf, 0},
		{"empty failOn fails on anything new", Delta{New: []string{"unknown"}}, "", sevOf, 1},
		{"nil sevOf treats keys as info", Delta{New: []string{"hi"}}, model.SevLow, nil, 0},
		{"nil sevOf with info threshold", Delta{New: []string{"hi"}}, model.SevInfo, nil, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Verdict(tc.d, tc.failOn, tc.sevOf); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}

func TestSeverityIndex(t *testing.T) {
	baseF := []model.Finding{f("k", model.StateTheorized, model.SevLow)}
	curF := []model.Finding{f("k", model.StateTheorized, model.SevMedium), f("k", model.StateTheorized, model.SevHigh), f("only-base", model.StateTheorized, model.SevInfo)}
	sevOf := SeverityIndex(baseF, curF)
	if got := sevOf("k"); got != model.SevHigh {
		t.Fatalf("k=%s", got)
	}
	if got := sevOf("missing"); got != "" {
		t.Fatalf("missing=%q", got)
	}
	// Earlier list still contributes keys the later one lacks.
	sevOf = SeverityIndex([]model.Finding{f("x", model.StateTheorized, model.SevCritical)}, curF)
	if got := sevOf("x"); got != model.SevCritical {
		t.Fatalf("x=%s", got)
	}
}

func TestMarkdown(t *testing.T) {
	d := Delta{BaselineRunID: "b", CurrentRunID: "c", New: []string{"n1"}, Fixed: []string{"f1", "f2"}, Regressed: []string{}, CostDeltaUSD: -0.25, P95DeltaMS: map[string]int64{"m2": 10, "m1": -5}}
	md := Markdown(d)
	for _, want := range []string{
		"## Continuous validation drift",
		"Baseline `b` compared with run `c`.",
		"| New | 1 |",
		"| Fixed | 2 |",
		"| Regressed | 0 |",
		"- `n1`",
		"- `f2`",
		"Spend delta: -0.2500 USD",
		"p95 latency `m1`: -5 ms\n- p95 latency `m2`: +10 ms",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in\n%s", want, md)
		}
	}
	if Markdown(d) != md {
		t.Fatal("not deterministic")
	}
	empty := Markdown(Delta{})
	if !strings.Contains(empty, "None.") || !strings.Contains(empty, "no model present in both runs") {
		t.Fatalf("empty body:\n%s", empty)
	}
}
