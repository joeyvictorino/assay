package reconcile

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func modelCall(seq uint64, agent, trace string, claims ...model.ClaimedToolCall) model.AuditRecord {
	// Mimic the shape after a JSON round trip through audit export.
	raw, _ := json.Marshal(claims)
	var anyClaims any
	_ = json.Unmarshal(raw, &anyClaims)
	return model.AuditRecord{Seq: seq, Kind: "model_call", Agent: agent, TraceID: trace, Meta: map[string]any{"claimed_tool_calls": anyClaims}}
}

func toolCall(seq uint64, agent, trace, id, tool, digest string) model.AuditRecord {
	return model.AuditRecord{Seq: seq, Kind: "tool_call", Agent: agent, TraceID: trace, ToolCallID: id, Tool: tool, ArgsDigest: digest}
}

func TestReconcileReasons(t *testing.T) {
	cases := []struct {
		name        string
		records     []model.AuditRecord
		wantChecked int
		want        []Mismatch
	}{
		{
			name: "clean match",
			records: []model.AuditRecord{
				modelCall(1, "probe", "t1", model.ClaimedToolCall{ID: "c1", Name: "http_get", ArgsDigest: "d1"}),
				toolCall(2, "probe", "t1", "c1", "http_get", "d1"),
			},
			wantChecked: 1,
			want:        []Mismatch{},
		},
		{
			name: "missing control plane event",
			records: []model.AuditRecord{
				modelCall(1, "probe", "t1", model.ClaimedToolCall{ID: "c1", Name: "http_get", ArgsDigest: "d1"}),
			},
			wantChecked: 1,
			want: []Mismatch{{
				ToolCallID: "c1", Reasons: []string{ReasonMissingControl},
				TranscriptSeqs: []uint64{1}, ControlSeqs: []uint64{}, Claimed: "http_get", Confidence: ConfidenceHigh,
			}},
		},
		{
			name: "missing transcript",
			records: []model.AuditRecord{
				toolCall(5, "probe", "t1", "c9", "http_post", "d9"),
			},
			wantChecked: 1,
			want: []Mismatch{{
				ToolCallID: "c9", Reasons: []string{ReasonMissingTranscript},
				TranscriptSeqs: []uint64{}, ControlSeqs: []uint64{5}, Actual: "http_post", Confidence: ConfidenceHigh,
			}},
		},
		{
			name: "duplicate transcript id lowers confidence",
			records: []model.AuditRecord{
				modelCall(3, "probe", "t1", model.ClaimedToolCall{ID: "c1", Name: "http_get", ArgsDigest: "d1"}),
				modelCall(1, "probe", "t1", model.ClaimedToolCall{ID: "c1", Name: "http_get", ArgsDigest: "d1"}),
				toolCall(2, "probe", "t1", "c1", "http_get", "d1"),
			},
			wantChecked: 1,
			want: []Mismatch{{
				ToolCallID: "c1", Reasons: []string{ReasonDuplicateTranscript},
				TranscriptSeqs: []uint64{1, 3}, ControlSeqs: []uint64{2}, Claimed: "http_get", Actual: "http_get", Confidence: ConfidenceMedium,
			}},
		},
		{
			name: "duplicate control id",
			records: []model.AuditRecord{
				modelCall(1, "probe", "t1", model.ClaimedToolCall{ID: "c1", Name: "http_get", ArgsDigest: "d1"}),
				toolCall(2, "probe", "t1", "c1", "http_get", "d1"),
				toolCall(4, "probe", "t1", "c1", "http_get", "d1"),
			},
			wantChecked: 1,
			want: []Mismatch{{
				ToolCallID: "c1", Reasons: []string{ReasonDuplicateControl},
				TranscriptSeqs: []uint64{1}, ControlSeqs: []uint64{2, 4}, Claimed: "http_get", Actual: "http_get", Confidence: ConfidenceMedium,
			}},
		},
		{
			name: "all four field mismatches in fixed order",
			records: []model.AuditRecord{
				modelCall(1, "probe", "t1", model.ClaimedToolCall{ID: "c1", Name: "http_get", ArgsDigest: "d1"}),
				toolCall(2, "recon", "t2", "c1", "http_post", "d2"),
			},
			wantChecked: 1,
			want: []Mismatch{{
				ToolCallID:     "c1",
				Reasons:        []string{ReasonAgentIDMismatch, ReasonTraceIDMismatch, ReasonToolNameMismatch, ReasonArgsDigestMismatch},
				TranscriptSeqs: []uint64{1}, ControlSeqs: []uint64{2}, Claimed: "http_get", Actual: "http_post", Confidence: ConfidenceHigh,
			}},
		},
		{
			name: "http sub-exchange records are not control events",
			records: []model.AuditRecord{
				modelCall(1, "probe", "", model.ClaimedToolCall{ID: "c1", Name: "http_get", ArgsDigest: "d1"}),
				toolCall(2, "probe", "", "c1", "http_get", "d1"),
				{Seq: 3, Kind: "tool_call", ToolCallID: "c1", Tool: "http", ArgsDigest: "x", Meta: map[string]any{"subkind": "http"}},
				{Seq: 4, Kind: "tool_call", ToolCallID: "c1", Tool: "http", ArgsDigest: "y", Meta: map[string]any{"subkind": "http"}},
			},
			wantChecked: 1,
			want:        []Mismatch{},
		},
		{
			name: "empty claimed id and unrelated kinds are ignored",
			records: []model.AuditRecord{
				modelCall(1, "probe", "", model.ClaimedToolCall{ID: "", Name: "http_get"}),
				{Seq: 2, Kind: "policy_decision", ToolCallID: "c1", Tool: "http_get"},
				{Seq: 3, Kind: "model_call"},
			},
			wantChecked: 0,
			want:        []Mismatch{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Reconcile(tc.records)
			if got.Checked != tc.wantChecked {
				t.Fatalf("checked=%d want %d", got.Checked, tc.wantChecked)
			}
			if !reflect.DeepEqual(got.Mismatches, tc.want) {
				t.Fatalf("mismatches=%+v\nwant %+v", got.Mismatches, tc.want)
			}
			wantCounts := map[string]int{}
			for _, m := range tc.want {
				for _, r := range m.Reasons {
					wantCounts[r]++
				}
			}
			if !reflect.DeepEqual(got.ReasonCounts, wantCounts) {
				t.Fatalf("counts=%v want %v", got.ReasonCounts, wantCounts)
			}
		})
	}
}

func TestReconcileOrderingIsDeterministic(t *testing.T) {
	recs := []model.AuditRecord{
		toolCall(9, "a", "", "zeta", "x", "1"),
		toolCall(7, "a", "", "alpha", "x", "1"),
		modelCall(3, "a", "", model.ClaimedToolCall{ID: "mid", Name: "y", ArgsDigest: "2"}),
	}
	first := Reconcile(recs)
	reversed := []model.AuditRecord{recs[2], recs[1], recs[0]}
	second := Reconcile(reversed)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("order-dependent result:\n%+v\n%+v", first, second)
	}
	var ids []string
	for _, m := range first.Mismatches {
		ids = append(ids, m.ToolCallID)
	}
	if !reflect.DeepEqual(ids, []string{"alpha", "mid", "zeta"}) {
		t.Fatalf("ids=%v", ids)
	}
}

func TestClaimedCallsShapes(t *testing.T) {
	typed := []model.ClaimedToolCall{{ID: "a", Name: "n", ArgsDigest: "d"}}
	maps := []map[string]any{{"id": "a", "name": "n", "args_digest": "d"}}
	cases := []struct {
		name string
		in   any
		want int
	}{
		{"typed slice", typed, 1},
		{"map slice", maps, 1},
		{"any slice", []any{map[string]any{"id": "a", "name": "n", "args_digest": "d"}}, 1},
		{"nil", nil, 0},
		{"garbage", "not a list", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(claimedCalls(tc.in)); got != tc.want {
				t.Fatalf("len=%d want %d", got, tc.want)
			}
		})
	}
}

func TestDowngrade(t *testing.T) {
	r := Report{Mismatches: []Mismatch{
		{ToolCallID: "c1", Reasons: []string{ReasonToolNameMismatch, ReasonArgsDigestMismatch}},
		{ToolCallID: "c2", Reasons: []string{ReasonMissingControl, ReasonToolNameMismatch}},
	}}
	cases := []struct {
		name        string
		in          model.Finding
		wantMatch   bool
		wantState   model.FindingState
		wantReasons []string
	}{
		{"no tool calls", model.Finding{ID: "f0", State: model.StateValidated}, true, model.StateValidated, nil},
		{"unrelated tool calls", model.Finding{ID: "f1", State: model.StateValidated, ToolCallIDs: []string{"c9"}}, true, model.StateValidated, nil},
		{"one mismatch", model.Finding{ID: "f2", State: model.StateValidated, ToolCallIDs: []string{"c9", "c1"}}, false, model.StateTheorized,
			[]string{ReasonArgsDigestMismatch, ReasonToolNameMismatch}},
		{"two mismatches union sorted dedup", model.Finding{ID: "f3", State: model.StateRefuted, ToolCallIDs: []string{"c1", "c2"}}, false, model.StateTheorized,
			[]string{ReasonArgsDigestMismatch, ReasonMissingControl, ReasonToolNameMismatch}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []model.Finding{tc.in}
			out := Downgrade(in, r)
			if len(out) != 1 {
				t.Fatalf("len=%d", len(out))
			}
			g := out[0]
			if !g.Reconcile.Checked || g.Reconcile.Match != tc.wantMatch || g.State != tc.wantState {
				t.Fatalf("got reconcile=%+v state=%s", g.Reconcile, g.State)
			}
			if !reflect.DeepEqual(g.Reconcile.Reasons, tc.wantReasons) {
				t.Fatalf("reasons=%v want %v", g.Reconcile.Reasons, tc.wantReasons)
			}
			if in[0].Reconcile.Checked {
				t.Fatal("input was mutated")
			}
		})
	}
}

func TestDowngradePreservesOrder(t *testing.T) {
	in := []model.Finding{{ID: "b"}, {ID: "a"}, {ID: "c"}}
	out := Downgrade(in, Report{})
	for i := range in {
		if out[i].ID != in[i].ID {
			t.Fatalf("order changed at %d: %s", i, out[i].ID)
		}
	}
}

func TestEncodeByteStable(t *testing.T) {
	a := Report{Checked: 2, Mismatches: []Mismatch{{ToolCallID: "c1", Reasons: []string{ReasonMissingControl}, TranscriptSeqs: []uint64{1}, Confidence: ConfidenceHigh}}, ReasonCounts: map[string]int{ReasonMissingControl: 1}}
	b := Report{Checked: 2, Mismatches: []Mismatch{{ToolCallID: "c1", Reasons: []string{ReasonMissingControl}, TranscriptSeqs: []uint64{1}, ControlSeqs: []uint64{}, Confidence: ConfidenceHigh}}, ReasonCounts: map[string]int{ReasonMissingControl: 1}}
	ea, err := Encode(a)
	if err != nil {
		t.Fatal(err)
	}
	eb, err := Encode(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ea, eb) {
		t.Fatalf("nil and empty slices encode differently:\n%s\n%s", ea, eb)
	}
	if !bytes.HasSuffix(ea, []byte("\n")) {
		t.Fatal("missing trailing newline")
	}
	if !strings.Contains(string(ea), `"control_seqs": []`) {
		t.Fatalf("expected empty control_seqs array: %s", ea)
	}
	empty, err := Encode(Report{})
	if err != nil {
		t.Fatal(err)
	}
	var back Report
	if err := json.Unmarshal(empty, &back); err != nil || back.Mismatches == nil || back.ReasonCounts == nil {
		t.Fatalf("empty report did not round-trip to empty containers: %s (%v)", empty, err)
	}
}

func TestAllReasonsCoverConstants(t *testing.T) {
	if len(AllReasons) != 8 {
		t.Fatalf("expected 8 reasons, got %d", len(AllReasons))
	}
	seen := map[string]bool{}
	for _, r := range AllReasons {
		if seen[r] {
			t.Fatalf("duplicate reason %s", r)
		}
		seen[r] = true
	}
}
