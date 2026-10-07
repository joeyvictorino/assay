// Package reconcile joins what models claimed (transcript side) against what
// the harness executed (control-plane side) and reports every disagreement.
//
// The transcript side is the claimed_tool_calls list the router records in
// every model_call audit record. The control-plane side is the tool_call
// record the agent loop writes for every executed call. Both carry the same
// tool call id, so the join is exact and needs no heuristics. The reason
// codes and the confidence rule are ported from orbit-ir's analyzer so the
// two projects' findings read the same.
//
// Everything here is deterministic: mismatches are sorted by tool call id,
// sequence numbers ascending, and Encode produces byte-stable JSON.
package reconcile

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/model"
)

// Reason codes, exactly as orbit-ir names them.
const (
	ReasonMissingControl      = "missing-control-plane-event"
	ReasonMissingTranscript   = "missing-transcript"
	ReasonDuplicateTranscript = "duplicate-transcript-tool-call-id"
	ReasonDuplicateControl    = "duplicate-control-tool-call-id"
	ReasonToolNameMismatch    = "tool-name-mismatch"
	ReasonArgsDigestMismatch  = "argument-digest-mismatch"
	ReasonAgentIDMismatch     = "agent-id-mismatch"
	ReasonTraceIDMismatch     = "trace-id-mismatch"
)

// Confidence values. Any duplicate-* reason lowers confidence to medium
// because the join is then ambiguous.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
)

const (
	kindModelCall = "model_call"
	kindToolCall  = "tool_call"
	metaClaimed   = "claimed_tool_calls"
	metaSubkind   = "subkind"
)

// AllReasons lists every reason code in report order.
var AllReasons = []string{
	ReasonMissingTranscript,
	ReasonMissingControl,
	ReasonDuplicateTranscript,
	ReasonDuplicateControl,
	ReasonAgentIDMismatch,
	ReasonTraceIDMismatch,
	ReasonToolNameMismatch,
	ReasonArgsDigestMismatch,
}

// Mismatch is one tool call id whose two sides disagree.
type Mismatch struct {
	ToolCallID     string   `json:"tool_call_id"`
	Reasons        []string `json:"reasons"`
	TranscriptSeqs []uint64 `json:"transcript_seqs"`
	ControlSeqs    []uint64 `json:"control_seqs"`
	Claimed        string   `json:"claimed"`
	Actual         string   `json:"actual"`
	Confidence     string   `json:"confidence"`
}

// Report is the reconciliation result for one run.
type Report struct {
	Checked      int            `json:"checked"`
	Mismatches   []Mismatch     `json:"mismatches"`
	ReasonCounts map[string]int `json:"reason_counts"`
}

// side is one observation of a tool call on either side of the join.
type side struct {
	seq     uint64
	tool    string
	digest  string
	agent   string
	traceID string
}

// claimedCalls decodes Meta["claimed_tool_calls"] whatever its in-memory
// shape is: []model.ClaimedToolCall or []map[string]any before the record
// was written, or []any of map[string]any after a JSON round trip through
// audit export.
func claimedCalls(v any) []model.ClaimedToolCall {
	if v == nil {
		return nil
	}
	if typed, ok := v.([]model.ClaimedToolCall); ok {
		return typed
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out []model.ClaimedToolCall
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

// isControlRecord reports whether rec is an executed-tool record. The
// gated HTTP client also writes tool_call records for each HTTP exchange a
// tool performs, tagged Meta["subkind"]="http" and carrying the parent tool
// call id; those are sub-exchanges, not executions, and are skipped here.
//
// LEAD: this relies on internal/httpx keeping the "subkind" meta key on its
// tool_call records. If that shape changes, mirror it here or every HTTP
// tool call will be reported as duplicate-control-tool-call-id.
func isControlRecord(rec model.AuditRecord) bool {
	if rec.Kind != kindToolCall || rec.ToolCallID == "" {
		return false
	}
	if rec.Meta != nil {
		if _, sub := rec.Meta[metaSubkind]; sub {
			return false
		}
	}
	return true
}

// Reconcile joins the transcript and control-plane sides of records and
// returns the sorted report. Records are taken as decrypted by audit.Export;
// input order does not affect the result. Claimed calls with an empty id
// cannot be joined and are ignored.
func Reconcile(records []model.AuditRecord) Report {
	transcripts := map[string][]side{}
	controls := map[string][]side{}
	for _, rec := range records {
		switch {
		case rec.Kind == kindModelCall:
			if rec.Meta == nil {
				continue
			}
			for _, c := range claimedCalls(rec.Meta[metaClaimed]) {
				if c.ID == "" {
					continue
				}
				transcripts[c.ID] = append(transcripts[c.ID], side{
					seq: rec.Seq, tool: c.Name, digest: c.ArgsDigest, agent: rec.Agent, traceID: rec.TraceID,
				})
			}
		case isControlRecord(rec):
			controls[rec.ToolCallID] = append(controls[rec.ToolCallID], side{
				seq: rec.Seq, tool: rec.Tool, digest: rec.ArgsDigest, agent: rec.Agent, traceID: rec.TraceID,
			})
		}
	}

	ids := make(map[string]bool, len(transcripts)+len(controls))
	for id := range transcripts {
		ids[id] = true
	}
	for id := range controls {
		ids[id] = true
	}
	sorted := make([]string, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)

	r := Report{Checked: len(sorted), Mismatches: []Mismatch{}, ReasonCounts: map[string]int{}}
	for _, id := range sorted {
		ts := sortSides(transcripts[id])
		cs := sortSides(controls[id])
		var reasons []string
		if len(ts) == 0 {
			reasons = append(reasons, ReasonMissingTranscript)
		}
		if len(cs) == 0 {
			reasons = append(reasons, ReasonMissingControl)
		}
		if len(ts) > 1 {
			reasons = append(reasons, ReasonDuplicateTranscript)
		}
		if len(cs) > 1 {
			reasons = append(reasons, ReasonDuplicateControl)
		}
		if len(ts) == 1 && len(cs) == 1 {
			t, c := ts[0], cs[0]
			if t.agent != c.agent {
				reasons = append(reasons, ReasonAgentIDMismatch)
			}
			if t.traceID != c.traceID {
				reasons = append(reasons, ReasonTraceIDMismatch)
			}
			if t.tool != c.tool {
				reasons = append(reasons, ReasonToolNameMismatch)
			}
			if t.digest != c.digest {
				reasons = append(reasons, ReasonArgsDigestMismatch)
			}
		}
		if len(reasons) == 0 {
			continue
		}
		m := Mismatch{
			ToolCallID:     id,
			Reasons:        reasons,
			TranscriptSeqs: seqs(ts),
			ControlSeqs:    seqs(cs),
			Confidence:     confidence(reasons),
		}
		if len(ts) > 0 {
			m.Claimed = ts[0].tool
		}
		if len(cs) > 0 {
			m.Actual = cs[0].tool
		}
		for _, reason := range reasons {
			r.ReasonCounts[reason]++
		}
		r.Mismatches = append(r.Mismatches, m)
	}
	return r
}

func sortSides(s []side) []side {
	out := append([]side(nil), s...)
	sort.Slice(out, func(i, j int) bool { return out[i].seq < out[j].seq })
	return out
}

func seqs(s []side) []uint64 {
	out := make([]uint64, 0, len(s))
	for _, x := range s {
		out = append(out, x.seq)
	}
	return out
}

func confidence(reasons []string) string {
	for _, r := range reasons {
		if strings.HasPrefix(r, "duplicate-") {
			return ConfidenceMedium
		}
	}
	return ConfidenceHigh
}

// Downgrade returns a copy of findings with Reconcile filled in. A finding
// whose ToolCallIDs intersect any mismatch gets Match=false, the union of
// the mismatch reasons (sorted, deduplicated) and State theorized; every
// other finding gets Checked=true, Match=true. Input order is preserved and
// the input slice is not modified.
func Downgrade(findings []model.Finding, r Report) []model.Finding {
	byID := make(map[string]Mismatch, len(r.Mismatches))
	for _, m := range r.Mismatches {
		byID[m.ToolCallID] = m
	}
	out := make([]model.Finding, len(findings))
	for i, f := range findings {
		g := f
		seen := map[string]bool{}
		var reasons []string
		for _, id := range f.ToolCallIDs {
			m, hit := byID[id]
			if !hit {
				continue
			}
			for _, reason := range m.Reasons {
				if !seen[reason] {
					seen[reason] = true
					reasons = append(reasons, reason)
				}
			}
		}
		if len(reasons) == 0 {
			g.Reconcile = model.ReconcileStatus{Checked: true, Match: true}
		} else {
			sort.Strings(reasons)
			g.Reconcile = model.ReconcileStatus{Checked: true, Match: false, Reasons: reasons}
			g.State = model.StateTheorized
		}
		out[i] = g
	}
	return out
}

// Encode renders r as indented JSON with a trailing newline. Nil containers
// are normalized to empty ones so the bytes do not depend on how the report
// was built.
func Encode(r Report) ([]byte, error) {
	if r.Mismatches == nil {
		r.Mismatches = []Mismatch{}
	}
	if r.ReasonCounts == nil {
		r.ReasonCounts = map[string]int{}
	}
	ms := make([]Mismatch, len(r.Mismatches))
	for i, m := range r.Mismatches {
		if m.Reasons == nil {
			m.Reasons = []string{}
		}
		if m.TranscriptSeqs == nil {
			m.TranscriptSeqs = []uint64{}
		}
		if m.ControlSeqs == nil {
			m.ControlSeqs = []uint64{}
		}
		ms[i] = m
	}
	r.Mismatches = ms
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
