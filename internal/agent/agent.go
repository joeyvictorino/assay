// Package agent runs the per-model tool loop: ask the router for a
// completion, verify and policy-check every tool call, execute it, feed all
// results back in one message, and stop on end_turn, refusal, error or the
// turn cap. Transcript bytes go only to the TranscriptSink; the audit log
// receives digests and bounded metadata.
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/joeyvictorino/assay/internal/finding"
	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/router"
	"github.com/joeyvictorino/assay/internal/tools"
)

// DefaultMaxTurns bounds the loop when Params.MaxTurns is zero.
const DefaultMaxTurns = 12

// Completer is the routing plane the agent talks to. *router.Router
// satisfies it.
type Completer interface {
	Complete(ctx context.Context, taskKind string, req model.Request) (model.Response, router.Decision, error)
}

// Params configures one agent run.
type Params struct {
	Router    Completer
	TaskKind  string
	TaskID    string
	Agent     string
	Lab       string
	RunID     string
	Objective string
	System    string
	Tools     []model.ToolManifest
	Verifier  model.ManifestVerifier
	Policy    model.PolicyEvaluator
	Auditor   model.Auditor
	Executor  tools.Executor
	Env       tools.Env
	MaxTurns  int
	MaxTokens int
	Sink      model.TranscriptSink
	// Depth and Ancestors describe the delegation chain for policy.
	Depth     int
	Ancestors []string
	// Now supplies timestamps; nil means time.Now.
	Now func() time.Time
}

// Result summarizes one agent run.
type Result struct {
	Findings   []model.Finding `json:"findings"`
	Declined   bool            `json:"declined"`
	Turns      int             `json:"turns"`
	Usage      model.Usage     `json:"usage"`
	Failovers  int             `json:"failovers"`
	StopReason string          `json:"stop_reason"` // end_turn | refusal | max_turns | max_tokens | error
	ToolCalls  int             `json:"tool_calls"`
	Denied     int             `json:"denied"`
}

var (
	// ErrNoRouter is returned when Params.Router is nil.
	ErrNoRouter = errors.New("agent: router is required")
	// ErrNoObjective is returned when Params.Objective is empty.
	ErrNoObjective = errors.New("agent: objective is required")
)

type run struct {
	p        Params
	now      func() time.Time
	findings map[string]model.Finding
	order    []string
	pending  []string // dedup keys reported during the current tool call
	result   Result
}

// Run executes the loop. The returned Result is populated even on error.
func Run(ctx context.Context, p Params) (Result, error) {
	if p.Router == nil {
		return Result{}, ErrNoRouter
	}
	if p.Objective == "" {
		return Result{}, ErrNoObjective
	}
	if p.MaxTurns <= 0 {
		p.MaxTurns = DefaultMaxTurns
	}
	if p.Executor == nil {
		p.Executor = tools.NewExecutor()
	}
	r := &run{p: p, now: p.Now, findings: map[string]model.Finding{}}
	if r.now == nil {
		r.now = time.Now
	}
	r.p.Env.Lab = p.Lab
	r.p.Env.RunID = p.RunID
	if r.p.Env.Model.Agent == "" {
		r.p.Env.Model.Agent = p.Agent
	}
	if r.p.Env.Now == nil {
		r.p.Env.Now = r.now
	}
	outerReport := p.Env.Report
	r.p.Env.Report = func(f model.Finding) {
		r.collect(f)
		if outerReport != nil {
			outerReport(f)
		}
	}
	err := r.loop(ctx)
	r.result.Findings = r.sortedFindings()
	return r.result, err
}

func (r *run) loop(ctx context.Context) error {
	msgs := []model.Message{{Role: "user", Content: r.p.Objective}}
	for turn := 0; turn < r.p.MaxTurns; turn++ {
		req := model.Request{
			System: r.p.System, Messages: msgs, Tools: r.p.Tools,
			MaxTokens: r.p.MaxTokens, TaskID: r.p.TaskID, Agent: r.p.Agent,
		}
		resp, dec, err := r.p.Router.Complete(ctx, r.p.TaskKind, req)
		r.result.Failovers += dec.Failovers
		if err != nil {
			r.result.StopReason = "error"
			return err
		}
		r.result.Turns++
		r.addUsage(resp.Usage)
		r.p.Env.Model.Provider, r.p.Env.Model.Model = splitChosen(dec.Chosen)
		r.sink(ctx, "assistant", resp)

		if resp.Declined {
			r.result.Declined = true
			r.result.StopReason = "refusal"
			return nil
		}
		msgs = append(msgs, model.Message{Role: "assistant", Content: resp.Text, ToolCalls: resp.ToolCalls})
		if len(resp.ToolCalls) == 0 {
			r.result.StopReason = resp.StopReason
			if r.result.StopReason == "" {
				r.result.StopReason = "end_turn"
			}
			return nil
		}
		results := r.handleToolCalls(ctx, resp.ToolCalls)
		r.sink(ctx, "tool", results)
		msgs = append(msgs, model.Message{Role: "user", ToolResults: results})
	}
	r.result.StopReason = "max_turns"
	return nil
}

func splitChosen(chosen string) (provider, modelID string) {
	for i := 0; i < len(chosen); i++ {
		if chosen[i] == '/' {
			return chosen[:i], chosen[i+1:]
		}
	}
	return "", chosen
}

func (r *run) handleToolCalls(ctx context.Context, calls []model.ToolCall) []model.ToolResult {
	results := make([]model.ToolResult, 0, len(calls))
	seen := map[string]bool{}
	for _, call := range calls {
		r.result.ToolCalls++
		digest := argsDigest(call.Args)
		switch {
		case call.ID == "":
			results = append(results, r.reject(ctx, call, digest, "invalid tool_call: missing id"))
			continue
		case seen[call.ID]:
			results = append(results, r.reject(ctx, call, digest, "invalid tool_call: duplicate id"))
			continue
		}
		seen[call.ID] = true

		manifest, ok := r.manifest(call.Name)
		if !ok {
			results = append(results, r.reject(ctx, call, digest, "unknown tool: "+call.Name))
			continue
		}
		dec, sigOK, signer := r.evaluate(ctx, manifest)
		r.auditPolicy(ctx, call, digest, dec, sigOK, signer)
		if dec.Effect != model.EffectAllow {
			r.result.Denied++
			reason := dec.Reason
			if reason == "" {
				reason = "policy"
			}
			results = append(results, model.ToolResult{ToolCallID: call.ID, Content: "denied: " + reason, IsError: true})
			continue
		}

		r.pending = nil
		start := r.now()
		res, err := r.p.Executor.Execute(ctx, call, r.p.Env)
		if err != nil {
			res = model.ToolResult{ToolCallID: call.ID, Content: "tool failed: " + err.Error(), IsError: true}
		}
		res.ToolCallID = call.ID
		seq := r.auditTool(ctx, call, digest, res, r.now().Sub(start).Milliseconds())
		r.attachControlPlane(seq)
		results = append(results, res)
	}
	return results
}

func (r *run) reject(ctx context.Context, call model.ToolCall, digest, msg string) model.ToolResult {
	res := model.ToolResult{ToolCallID: call.ID, Content: msg, IsError: true}
	r.auditTool(ctx, call, digest, res, 0)
	return res
}

func (r *run) manifest(name string) (model.ToolManifest, bool) {
	for _, m := range r.p.Tools {
		if m.Name == name {
			return m, true
		}
	}
	return model.ToolManifest{}, false
}

func (r *run) evaluate(ctx context.Context, m model.ToolManifest) (model.Decision, bool, string) {
	sigOK := false
	sigReason := "no_verifier"
	signer := ""
	if r.p.Verifier != nil {
		keyID, reason, err := r.p.Verifier.Verify(m)
		if err == nil {
			sigOK = true
			sigReason = ""
			signer = keyID
		} else {
			sigReason = reason
			if sigReason == "" {
				sigReason = err.Error()
			}
		}
	}
	if r.p.Policy == nil {
		return model.Decision{Effect: model.EffectDeny, Reason: "DENY_NO_POLICY", Rationale: "no policy evaluator configured"}, sigOK, signer
	}
	dec := r.p.Policy.Evaluate(ctx, model.PolicyRequest{
		Agent: r.p.Agent, Tool: m, Lab: r.p.Lab, Depth: r.p.Depth, Ancestors: r.p.Ancestors,
		SpentUSD: r.result.Usage.CostUSD, SignatureOK: sigOK, SignatureReason: sigReason,
	})
	return dec, sigOK, signer
}

func (r *run) collect(f model.Finding) {
	key := f.DedupKey
	if key == "" {
		key = finding.DedupKey(f.Lab, f.Class, f.Location.Method, f.Location.PathTemplate, f.Location.Param)
		f.DedupKey = key
	}
	if existing, ok := r.findings[key]; ok {
		r.findings[key] = finding.Merge(existing, f)
	} else {
		r.findings[key] = f
		r.order = append(r.order, key)
	}
	r.pending = append(r.pending, key)
}

func (r *run) attachControlPlane(seq uint64) {
	if seq == 0 {
		return
	}
	for _, key := range r.pending {
		f := r.findings[key]
		f.ControlPlane = append(f.ControlPlane, model.AuditRef{RunID: r.p.RunID, Seq: seq})
		r.findings[key] = f
	}
	r.pending = nil
}

func (r *run) sortedFindings() []model.Finding {
	out := make([]model.Finding, 0, len(r.order))
	for _, k := range r.order {
		out = append(out, r.findings[k])
	}
	return out
}

func (r *run) addUsage(u model.Usage) {
	r.result.Usage.InputTokens += u.InputTokens
	r.result.Usage.OutputTokens += u.OutputTokens
	r.result.Usage.CacheReadTokens += u.CacheReadTokens
	r.result.Usage.CacheWriteTokens += u.CacheWriteTokens
	r.result.Usage.CostUSD += u.CostUSD
	r.result.Usage.LatencyMS += u.LatencyMS
}

// sink writes transcript material. It is the only place response text or
// tool-result bodies leave this package.
func (r *run) sink(ctx context.Context, role string, body any) {
	if r.p.Sink == nil {
		return
	}
	b, err := json.Marshal(map[string]any{"role": role, "time": r.now().UTC(), "body": body})
	if err != nil {
		return
	}
	_ = r.p.Sink.Write(ctx, r.p.RunID, r.p.Agent, b)
}

func (r *run) auditPolicy(ctx context.Context, call model.ToolCall, digest string, dec model.Decision, sigOK bool, signer string) {
	if r.p.Auditor == nil {
		return
	}
	rules := append([]string(nil), dec.MatchedRules...)
	if rules == nil {
		rules = []string{}
	}
	meta := map[string]any{
		"effect":        string(dec.Effect),
		"reason":        dec.Reason,
		"rationale":     dec.Rationale,
		"matched_rules": rules,
		"policy_hash":   dec.PolicyHash,
		"signature_ok":  sigOK,
		"signer":        signer,
		"task_kind":     r.p.TaskKind,
	}
	_, _ = r.p.Auditor.Record(ctx, model.AuditRecord{
		Time: r.now().UTC(), RunID: r.p.RunID, TaskID: r.p.TaskID, Agent: r.p.Agent,
		Kind: "policy_decision", ToolCallID: call.ID, Tool: call.Name, ArgsDigest: digest, Meta: meta,
	})
}

func (r *run) auditTool(ctx context.Context, call model.ToolCall, digest string, res model.ToolResult, latencyMS int64) uint64 {
	if r.p.Auditor == nil {
		return 0
	}
	seq, _ := r.p.Auditor.Record(ctx, model.AuditRecord{
		Time: r.now().UTC(), RunID: r.p.RunID, TaskID: r.p.TaskID, Agent: r.p.Agent,
		Kind: "tool_call", ToolCallID: call.ID, Tool: call.Name, ArgsDigest: digest,
		Hashes: map[string]string{"result": sha256Hex([]byte(res.Content))},
		Meta: map[string]any{
			"is_error":     res.IsError,
			"result_bytes": len(res.Content),
			"latency_ms":   latencyMS,
			"task_kind":    r.p.TaskKind,
		},
	})
	return seq
}

func argsDigest(args map[string]any) string {
	if args == nil {
		args = map[string]any{}
	}
	b, _ := json.Marshal(args)
	return sha256Hex(b)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// String renders a compact result summary for logs. It never includes
// finding summaries or transcript text.
func (r Result) String() string {
	return fmt.Sprintf("turns=%d tool_calls=%d denied=%d findings=%d declined=%v stop=%s cost_usd=%.6f",
		r.Turns, r.ToolCalls, r.Denied, len(r.Findings), r.Declined, r.StopReason, r.Usage.CostUSD)
}
