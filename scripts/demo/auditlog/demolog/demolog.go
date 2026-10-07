// Package demolog writes a short, realistic sequence of audit records for
// the demo recording. Records carry digests and bounded metadata only, as
// the audit writer requires; nothing here is a real run.
package demolog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/joeyvictorino/assay/internal/audit"
	"github.com/joeyvictorino/assay/internal/model"
)

func digest(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Records returns the demo record sequence for runID. Times are fixed so
// two demo logs differ only in nonces and ciphertext.
func Records(runID string) []model.AuditRecord {
	t0 := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }
	argsGet := digest(`{"url":"http://127.0.0.1:9000/api/users/1"}`)
	return []model.AuditRecord{
		{Time: at(0), RunID: runID, Kind: "gate_decision", Agent: "orchestrator",
			Meta: map[string]any{"effect": "allow", "reason": "SCOPE_OK", "scope_key_id": "demo-key-id"}},
		{Time: at(1), RunID: runID, TaskID: "t-1", Agent: "recon", Kind: "router_decision",
			Meta: map[string]any{"task_kind": "recon", "chosen": "fake-model", "reason": "first-candidate-with-budget"}},
		{Time: at(2), RunID: runID, TaskID: "t-1", Agent: "recon", Kind: "model_call",
			Hashes: map[string]string{"prompt": digest("prompt-1"), "response": digest("response-1")},
			Meta: map[string]any{"model": "fake-model", "tokens": 512, "latency_ms": 840, "cost_microusd": 1200, "declined": false,
				"claimed_tool_calls": []map[string]any{{"id": "call-1", "name": "http_get", "args_digest": argsGet}}}},
		{Time: at(3), RunID: runID, TaskID: "t-1", Agent: "recon", Kind: "policy_decision", ToolCallID: "call-1", Tool: "http_get", ArgsDigest: argsGet,
			Meta: map[string]any{"effect": "allow", "reason": "ALLOW_RULE:recon-read", "signature_ok": true}},
		{Time: at(4), RunID: runID, TaskID: "t-1", Agent: "recon", Kind: "tool_call", ToolCallID: "call-1", Tool: "http_get", ArgsDigest: argsGet,
			Hashes: map[string]string{"result": digest("result-1")},
			Meta:   map[string]any{"is_error": false, "result_bytes": 2048, "latency_ms": 31}},
		{Time: at(5), RunID: runID, TaskID: "t-1", Agent: "recon", Kind: "finding",
			Meta: map[string]any{"id": "f-demo000000000001", "class": "info-disclosure", "state": "theorized"}},
	}
}

// Write creates the log at path, appends Records(runID) and returns the
// verification summary computed with the same key.
func Write(path, runID string, master []byte) (audit.Summary, error) {
	w, err := audit.OpenWriter(path, runID, master)
	if err != nil {
		return audit.Summary{}, err
	}
	for _, rec := range Records(runID) {
		if _, err := w.Record(context.Background(), rec); err != nil {
			w.Close()
			return audit.Summary{}, err
		}
	}
	if err := w.Close(); err != nil {
		return audit.Summary{}, err
	}
	return audit.Verify(path, master)
}
