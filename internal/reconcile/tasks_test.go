package reconcile

import (
	"reflect"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func inTask(rec model.AuditRecord, task string) model.AuditRecord {
	rec.TaskID = task
	return rec
}

func claim(id, name, digest string) model.ClaimedToolCall {
	return model.ClaimedToolCall{ID: id, Name: name, ArgsDigest: digest}
}

// The same id in two tasks (two labs, or two models) is not a reuse: ids are
// only unique within one conversation.
func TestSameIDInDifferentTasksIsClean(t *testing.T) {
	var recs []model.AuditRecord
	seq := uint64(1)
	for _, task := range []string{"lab-a:m1", "lab-b:m1", "lab-a:m2"} {
		recs = append(recs,
			inTask(modelCall(seq, "probe", "", claim("call_0", "http_get", "d1")), task),
			inTask(toolCall(seq+1, "probe", "", "call_0", "http_get", "d1"), task))
		seq += 2
	}
	r := Reconcile(recs)
	if len(r.Mismatches) != 0 || r.Checked != 3 {
		t.Fatalf("checked=%d mismatches=%+v", r.Checked, r.Mismatches)
	}
}

// Reuse inside one task is genuinely ambiguous and stays a duplicate with
// medium confidence, carrying the task id.
func TestReuseInsideOneTaskIsFlagged(t *testing.T) {
	task := "lab-a:m1"
	recs := []model.AuditRecord{
		inTask(modelCall(1, "probe", "", claim("call_0", "http_get", "d1")), task),
		inTask(toolCall(2, "probe", "", "call_0", "http_get", "d1"), task),
		inTask(modelCall(3, "probe", "", claim("call_0", "http_get", "d2")), task),
		inTask(toolCall(4, "probe", "", "call_0", "http_get", "d2"), task),
	}
	r := Reconcile(recs)
	if len(r.Mismatches) != 1 {
		t.Fatalf("%+v", r.Mismatches)
	}
	m := r.Mismatches[0]
	want := []string{ReasonDuplicateTranscript, ReasonDuplicateControl}
	if m.TaskID != task || m.ToolCallID != "call_0" || !reflect.DeepEqual(m.Reasons, want) || m.Confidence != ConfidenceMedium {
		t.Fatalf("%+v", m)
	}
}

// A derived control id joins on the claimed id recorded beside it.
func TestDerivedControlIDJoinsOnClaimedID(t *testing.T) {
	task := "lab-a:m1"
	derived := inTask(toolCall(4, "probe", "", "call_0#2", "http_get", "d2"), task)
	derived.Meta = map[string]any{metaClaimedID: "call_0"}
	recs := []model.AuditRecord{
		inTask(modelCall(1, "probe", "", claim("call_0", "http_get", "d1")), task),
		inTask(toolCall(2, "probe", "", "call_0", "http_get", "d1"), task),
		inTask(modelCall(3, "probe", "", claim("call_0", "http_get", "d2")), task),
		derived,
	}
	r := Reconcile(recs)
	if r.Checked != 1 {
		t.Fatalf("derived id must not create a separate join key: checked=%d %+v", r.Checked, r.Mismatches)
	}
	if len(r.Mismatches) != 1 || !reflect.DeepEqual(r.Mismatches[0].Reasons, []string{ReasonDuplicateTranscript, ReasonDuplicateControl}) {
		t.Fatalf("%+v", r.Mismatches)
	}
}

// A mismatch in one lab must not downgrade a finding from another lab that
// cites the same id.
func TestDowngradeIsScopedToTheFindingsLab(t *testing.T) {
	r := Report{Mismatches: []Mismatch{{
		TaskID: "lab-a:m1", ToolCallID: "call_0", Reasons: []string{ReasonMissingControl}, Confidence: ConfidenceHigh,
	}}}
	fs := []model.Finding{
		{ID: "a", Lab: "lab-a", State: model.StateValidated, ToolCallIDs: []string{"call_0"}},
		{ID: "b", Lab: "lab-b", State: model.StateValidated, ToolCallIDs: []string{"call_0"}},
	}
	out := Downgrade(fs, r)
	if out[0].State != model.StateTheorized || out[0].Reconcile.Match {
		t.Fatalf("lab-a finding should be downgraded: %+v", out[0])
	}
	if out[1].State != model.StateValidated || !out[1].Reconcile.Match {
		t.Fatalf("lab-b finding must be untouched: %+v", out[1])
	}
}

// Records without task ids (older logs) keep the original behavior.
func TestRecordsWithoutTaskIDsStillJoin(t *testing.T) {
	r := Reconcile([]model.AuditRecord{
		modelCall(1, "probe", "", claim("x", "http_get", "d")),
		toolCall(2, "probe", "", "x", "http_get", "d"),
	})
	if len(r.Mismatches) != 0 || r.Checked != 1 {
		t.Fatalf("%+v", r)
	}
}

func policyRecord(seq uint64, agent, id, tool, effect string) model.AuditRecord {
	return model.AuditRecord{Seq: seq, Kind: "policy_decision", Agent: agent, ToolCallID: id, Tool: tool,
		Meta: map[string]any{"effect": effect}}
}

func TestDeniedClaimIsAccountedForNotMismatched(t *testing.T) {
	task := "lab-a:m1"
	recs := []model.AuditRecord{
		inTask(modelCall(1, "probe", "", claim("call_9", "delegate", "d")), task),
		inTask(policyRecord(2, "probe", "call_9", "delegate", "deny"), task),
	}
	r := Reconcile(recs)
	if len(r.Mismatches) != 0 {
		t.Fatalf("a denied call is not a mismatch: %+v", r.Mismatches)
	}
	if len(r.Denied) != 1 || r.Denied[0].Tool != "delegate" || r.Denied[0].ToolCallID != "call_9" || r.Denied[0].TaskID != task {
		t.Fatalf("denied: %+v", r.Denied)
	}
}

func TestClaimWithNoRecordAtAllIsStillAMismatch(t *testing.T) {
	r := Reconcile([]model.AuditRecord{
		inTask(modelCall(1, "probe", "", claim("call_9", "delegate", "d")), "lab-a:m1"),
	})
	if len(r.Mismatches) != 1 || r.Mismatches[0].Reasons[0] != ReasonMissingControl || len(r.Denied) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestAllowedButNeverExecutedIsStillAMismatch(t *testing.T) {
	task := "lab-a:m1"
	r := Reconcile([]model.AuditRecord{
		inTask(modelCall(1, "probe", "", claim("call_9", "http_get", "d")), task),
		inTask(policyRecord(2, "probe", "call_9", "http_get", "allow"), task),
	})
	if len(r.Mismatches) != 1 || len(r.Denied) != 0 {
		t.Fatalf("an allow with no execution is suspicious: %+v", r)
	}
}

func TestDenialForADifferentToolDoesNotCoverTheClaim(t *testing.T) {
	task := "lab-a:m1"
	r := Reconcile([]model.AuditRecord{
		inTask(modelCall(1, "probe", "", claim("call_9", "http_get", "d")), task),
		inTask(policyRecord(2, "probe", "call_9", "delegate", "deny"), task),
	})
	if len(r.Mismatches) != 1 || len(r.Denied) != 0 {
		t.Fatalf("%+v", r)
	}
}

func TestDenialInAnotherTaskDoesNotCoverTheClaim(t *testing.T) {
	r := Reconcile([]model.AuditRecord{
		inTask(modelCall(1, "probe", "", claim("call_9", "delegate", "d")), "lab-a:m1"),
		inTask(policyRecord(2, "probe", "call_9", "delegate", "deny"), "lab-b:m1"),
	})
	if len(r.Mismatches) != 1 || len(r.Denied) != 0 {
		t.Fatalf("%+v", r)
	}
}
