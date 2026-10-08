package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/provider"
	"github.com/joeyvictorino/assay/internal/provider/fake"
	"github.com/joeyvictorino/assay/internal/router"
	"github.com/joeyvictorino/assay/internal/tools"
)

// --- fakes for the interfaces other streams implement ---

type memAuditor struct {
	mu   sync.Mutex
	recs []model.AuditRecord
}

func (a *memAuditor) Record(_ context.Context, rec model.AuditRecord) (uint64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	rec.Seq = uint64(len(a.recs) + 1)
	a.recs = append(a.recs, rec)
	return rec.Seq, nil
}

func (a *memAuditor) kinds(kind string) []model.AuditRecord {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []model.AuditRecord
	for _, r := range a.recs {
		if r.Kind == kind {
			out = append(out, r)
		}
	}
	return out
}

func (a *memAuditor) json(t *testing.T) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, err := json.Marshal(a.recs)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type memSink struct {
	mu   sync.Mutex
	body []byte
}

func (s *memSink) Write(_ context.Context, _, _ string, b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.body = append(s.body, b...)
	s.body = append(s.body, '\n')
	return nil
}

type okVerifier struct{}

func (okVerifier) Verify(model.ToolManifest) (string, string, error) { return "key-1", "", nil }

type badVerifier struct{}

func (badVerifier) Verify(model.ToolManifest) (string, string, error) {
	return "", "SIG_MISSING", errors.New("unsigned")
}

// denyPolicy denies the tools in deny and requires a valid signature.
type denyPolicy struct {
	deny map[string]bool
	seen []model.PolicyRequest
	mu   sync.Mutex
}

func (p *denyPolicy) Evaluate(_ context.Context, req model.PolicyRequest) model.Decision {
	p.mu.Lock()
	p.seen = append(p.seen, req)
	p.mu.Unlock()
	if !req.SignatureOK {
		return model.Decision{Effect: model.EffectDeny, Reason: "DENY_UNSIGNED:" + req.SignatureReason}
	}
	if p.deny[req.Tool.Name] {
		return model.Decision{Effect: model.EffectDeny, Reason: "DENY_RULE:no-write", Rationale: "writes forbidden", MatchedRules: []string{"no-write"}}
	}
	return model.Decision{Effect: model.EffectAllow, Reason: "ALLOW_DEFAULT", PolicyHash: "ph"}
}

type allowGate struct{}

func (allowGate) Allow(context.Context, string) model.Decision {
	return model.Decision{Effect: model.EffectAllow, Reason: "ALLOW_SCOPE"}
}

// --- helpers ---

var costs = provider.Costs{"fake-model": {InputUSD: 1, OutputUSD: 5, CacheReadUSD: 0.1}}

func newRouter(aud model.Auditor, p model.Provider) *router.Router {
	reg := provider.NewRegistry()
	reg.Register("fake", p)
	return router.New(router.Config{
		Routes:  map[string][]model.ModelRef{"probe": {{Provider: "fake", Model: "fake-model", Agent: "probe"}}},
		Budgets: router.Budgets{PerRun: model.Budget{MaxUSD: 10}, PerModel: model.Budget{MaxUSD: 5}, PerTask: model.Budget{MaxUSD: 1, MaxLatencyMS: 5000}},
		Costs:   costs,
		RunID:   "run-1",
	}, reg, aud)
}

func manifests(t *testing.T) []model.ToolManifest {
	t.Helper()
	ms, err := tools.Manifests()
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

func httpOK(body string) tools.HTTPFunc {
	return func(context.Context, string, string, map[string]string, []byte) (int, map[string]string, []byte, error) {
		return 200, map[string]string{"Server": "lab"}, []byte(body), nil
	}
}

func params(t *testing.T, aud *memAuditor, sink model.TranscriptSink, p model.Provider, pol model.PolicyEvaluator, ver model.ManifestVerifier) Params {
	return Params{
		Router: newRouter(aud, p), TaskKind: "probe", TaskID: "t1", Agent: "probe", Lab: "lab-a", RunID: "run-1",
		Objective: "Probe /api/users for IDOR.", System: "You are a probe agent.", Tools: manifests(t),
		Verifier: ver, Policy: pol, Auditor: aud, Executor: tools.NewExecutor(),
		Env:      tools.Env{HTTP: httpOK(`{"user":"bob","email":"bob@lab"}`), Gate: allowGate{}},
		MaxTurns: 6, MaxTokens: 512, Sink: sink,
	}
}

func getCall(id, url string) model.ToolCall {
	return model.ToolCall{ID: id, Name: "http_get", Args: map[string]any{"url": url}}
}

func reportCall(id string) model.ToolCall {
	return model.ToolCall{ID: id, Name: "report_finding", Args: map[string]any{
		"class": "idor", "method": "GET", "path": "/api/users/42", "param": "id", "severity": "high",
		"summary": "User 42 readable without authorization.", "evidence_tool_call_ids": []any{"c1"},
	}}
}

// --- tests ---

func TestHappyPathProducesFinding(t *testing.T) {
	aud := &memAuditor{}
	sink := &memSink{}
	prov := fake.New([]fake.Step{
		fake.ToolCalls(getCall("c1", "http://lab/api/users/42"), getCall("c2", "http://lab/api/users/43")),
		fake.ToolCalls(reportCall("c3")),
		fake.Text("Done."),
	}, fake.Options{})
	pol := &denyPolicy{deny: map[string]bool{}}
	res, err := Run(context.Background(), params(t, aud, sink, prov, pol, okVerifier{}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Turns != 3 || res.ToolCalls != 3 || res.Denied != 0 || res.Declined || res.StopReason != "end_turn" {
		t.Fatalf("result = %s", res)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("findings = %d", len(res.Findings))
	}
	f := res.Findings[0]
	if f.Class != model.ClassIDOR || f.Location.PathTemplate != "/api/users/{id}" || f.State != model.StateTheorized || f.Lab != "lab-a" || f.RunID != "run-1" {
		t.Fatalf("finding = %+v", f)
	}
	if f.Model.Provider != "fake" || f.Model.Model != "fake-model" || f.Model.Agent != "probe" {
		t.Fatalf("finding model ref = %+v", f.Model)
	}
	if len(f.ControlPlane) != 1 || f.ControlPlane[0].RunID != "run-1" || f.ControlPlane[0].Seq == 0 {
		t.Fatalf("control plane = %+v", f.ControlPlane)
	}
	if res.Usage.InputTokens == 0 || res.Usage.CostUSD == 0 {
		t.Fatalf("usage = %+v", res.Usage)
	}

	// Tool results for a turn were returned in a single message.
	calls := prov.Calls()
	if len(calls) != 3 {
		t.Fatalf("provider calls = %d", len(calls))
	}
	second := calls[1].Messages
	if len(second) != 3 || second[2].Role != "user" || len(second[2].ToolResults) != 2 {
		t.Fatalf("turn-2 messages = %+v", second)
	}
	if second[2].ToolResults[0].ToolCallID != "c1" || second[2].ToolResults[1].ToolCallID != "c2" {
		t.Fatalf("results order = %+v", second[2].ToolResults)
	}
	if second[1].Role != "assistant" || len(second[1].ToolCalls) != 2 {
		t.Fatalf("assistant turn = %+v", second[1])
	}

	// Audit: 3 tool_call, 3 policy_decision, 3 model_call, 3 router_decision.
	tc := aud.kinds("tool_call")
	if len(tc) != 3 {
		t.Fatalf("tool_call records = %d", len(tc))
	}
	if tc[0].Tool != "http_get" || tc[0].ToolCallID != "c1" || len(tc[0].ArgsDigest) != 64 || len(tc[0].Hashes["result"]) != 64 {
		t.Fatalf("tool_call = %+v", tc[0])
	}
	if tc[0].Meta["is_error"] != false {
		t.Fatalf("tool_call meta = %v", tc[0].Meta)
	}
	pd := aud.kinds("policy_decision")
	if len(pd) != 3 || pd[0].Meta["effect"] != "allow" || pd[0].Meta["signature_ok"] != true || pd[0].Meta["signer"] != "key-1" {
		t.Fatalf("policy_decision = %+v", pd)
	}
	if len(aud.kinds("model_call")) != 3 || len(aud.kinds("router_decision")) != 3 {
		t.Fatal("model_call / router_decision counts")
	}
	// Bodies never reach the audit log.
	if s := aud.json(t); strings.Contains(s, "bob@lab") || strings.Contains(s, "Probe /api/users") || strings.Contains(s, "readable without") {
		t.Fatalf("audit log contains body material: %s", s)
	}
	// Transcript bytes went to the sink.
	if !strings.Contains(string(sink.body), "bob@lab") || !strings.Contains(string(sink.body), "Done.") {
		t.Fatalf("sink missing transcript: %s", sink.body)
	}
	// Policy saw the right request shape.
	if pol.seen[0].Agent != "probe" || pol.seen[0].Lab != "lab-a" || pol.seen[0].Tool.Name != "http_get" || !pol.seen[0].SignatureOK {
		t.Fatalf("policy request = %+v", pol.seen[0])
	}
}

func TestDeniedToolReturnsErrorResult(t *testing.T) {
	aud := &memAuditor{}
	prov := fake.New([]fake.Step{
		fake.ToolCalls(model.ToolCall{ID: "p1", Name: "http_post", Args: map[string]any{"url": "http://lab/x", "content_type": "text/plain", "body": "x"}}, getCall("g1", "http://lab/y")),
		fake.Text("ok"),
	}, fake.Options{})
	var httpCalls int
	p := params(t, aud, nil, prov, &denyPolicy{deny: map[string]bool{"http_post": true}}, okVerifier{})
	p.Env.HTTP = func(ctx context.Context, m, u string, h map[string]string, b []byte) (int, map[string]string, []byte, error) {
		httpCalls++
		return 200, nil, []byte("y"), nil
	}
	res, err := Run(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if res.Denied != 1 || res.ToolCalls != 2 || httpCalls != 1 {
		t.Fatalf("result = %s httpCalls=%d", res, httpCalls)
	}
	results := prov.Calls()[1].Messages[2].ToolResults
	if len(results) != 2 || !results[0].IsError || results[0].Content != "denied: DENY_RULE:no-write" || results[0].ToolCallID != "p1" {
		t.Fatalf("results = %+v", results)
	}
	if results[1].IsError {
		t.Fatalf("allowed call errored: %+v", results[1])
	}
	pd := aud.kinds("policy_decision")
	if len(pd) != 2 || pd[0].Meta["effect"] != "deny" || pd[0].Meta["reason"] != "DENY_RULE:no-write" || pd[0].Tool != "http_post" {
		t.Fatalf("policy_decision = %+v", pd)
	}
	if rules := pd[0].Meta["matched_rules"].([]string); len(rules) != 1 || rules[0] != "no-write" {
		t.Fatalf("matched_rules = %v", pd[0].Meta["matched_rules"])
	}
	// Denied calls get no tool_call record (nothing executed), allowed ones do.
	if tc := aud.kinds("tool_call"); len(tc) != 1 || tc[0].ToolCallID != "g1" {
		t.Fatalf("tool_call records = %+v", tc)
	}
}

func TestUnsignedManifestIsDenied(t *testing.T) {
	aud := &memAuditor{}
	prov := fake.New([]fake.Step{fake.ToolCalls(getCall("g1", "http://lab/y")), fake.Text("ok")}, fake.Options{})
	res, err := Run(context.Background(), params(t, aud, nil, prov, &denyPolicy{}, badVerifier{}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Denied != 1 {
		t.Fatalf("result = %s", res)
	}
	r := prov.Calls()[1].Messages[2].ToolResults[0]
	if !r.IsError || r.Content != "denied: DENY_UNSIGNED:SIG_MISSING" {
		t.Fatalf("result = %+v", r)
	}
	pd := aud.kinds("policy_decision")
	if pd[0].Meta["signature_ok"] != false {
		t.Fatalf("policy_decision = %+v", pd[0].Meta)
	}
}

func TestNilPolicyFailsClosed(t *testing.T) {
	prov := fake.New([]fake.Step{fake.ToolCalls(getCall("g1", "http://lab/y")), fake.Text("ok")}, fake.Options{})
	p := params(t, &memAuditor{}, nil, prov, nil, okVerifier{})
	res, err := Run(context.Background(), p)
	if err != nil || res.Denied != 1 {
		t.Fatalf("res=%s err=%v", res, err)
	}
	if r := prov.Calls()[1].Messages[2].ToolResults[0]; !strings.HasPrefix(r.Content, "denied: DENY_NO_POLICY") {
		t.Fatalf("result = %+v", r)
	}
}

func TestRefusalStopsImmediately(t *testing.T) {
	aud := &memAuditor{}
	prov := fake.New([]fake.Step{fake.ToolCalls(getCall("c1", "http://lab/a")), fake.Refusal(), fake.Text("never")}, fake.Options{})
	res, err := Run(context.Background(), params(t, aud, nil, prov, &denyPolicy{}, okVerifier{}))
	if err != nil {
		t.Fatalf("refusal must not be an error: %v", err)
	}
	if !res.Declined || res.StopReason != "refusal" || res.Turns != 2 {
		t.Fatalf("result = %s", res)
	}
	if prov.Remaining() != 1 {
		t.Fatal("agent continued after refusal")
	}
	if len(res.Findings) != 0 {
		t.Fatal("refusal produced findings")
	}
	mc := aud.kinds("model_call")
	if len(mc) != 2 || mc[1].Meta["declined"] != true {
		t.Fatalf("model_call = %+v", mc)
	}
}

func TestRefusalOnFirstTurn(t *testing.T) {
	prov := fake.New([]fake.Step{fake.Text("x")}, fake.Options{Refuse: true})
	res, err := Run(context.Background(), params(t, &memAuditor{}, nil, prov, &denyPolicy{}, okVerifier{}))
	if err != nil || !res.Declined || res.Turns != 1 || res.ToolCalls != 0 {
		t.Fatalf("res=%s err=%v", res, err)
	}
}

func TestCanaryNeverLeaksIntoResult(t *testing.T) {
	const canary = "CANARY-7f3a9c-DO-NOT-LEAK"
	aud := &memAuditor{}
	sink := &memSink{}
	prov := fake.New([]fake.Step{
		fake.ToolCalls(getCall("c1", "http://lab/api/users/1")),
		fake.ToolCalls(reportCall("c2")),
		fake.Text("final"),
	}, fake.Options{Canary: canary})
	p := params(t, aud, sink, prov, &denyPolicy{}, okVerifier{})
	p.Env.HTTP = httpOK("body with " + canary)
	res, err := Run(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("findings = %d", len(res.Findings))
	}
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), canary) {
		t.Fatalf("canary leaked into result: %s", b)
	}
	if strings.Contains(res.String(), canary) {
		t.Fatal("canary leaked into result string")
	}
	if strings.Contains(aud.json(t), canary) {
		t.Fatal("canary leaked into audit log")
	}
	if !strings.Contains(string(sink.body), canary) {
		t.Fatal("transcript sink should be the only place the canary appears")
	}
}

func TestSanitizeHookAppliedToSummary(t *testing.T) {
	prov := fake.New([]fake.Step{fake.ToolCalls(reportCall("c2")), fake.Text("done")}, fake.Options{})
	p := params(t, &memAuditor{}, nil, prov, &denyPolicy{}, okVerifier{})
	p.Env.Sanitize = func(s string) string { return strings.ReplaceAll(s, "authorization", "[redacted]") }
	res, err := Run(context.Background(), p)
	if err != nil || len(res.Findings) != 1 {
		t.Fatalf("res=%s err=%v", res, err)
	}
	if !strings.Contains(res.Findings[0].Summary, "[redacted]") {
		t.Fatalf("summary not sanitized: %q", res.Findings[0].Summary)
	}
}

func TestOversizeSummaryIsBounded(t *testing.T) {
	prov := fake.New([]fake.Step{fake.ToolCalls(reportCall("c2")), fake.Text("done")}, fake.Options{OversizeSummary: true})
	res, err := Run(context.Background(), params(t, &memAuditor{}, nil, prov, &denyPolicy{}, okVerifier{}))
	if err != nil || len(res.Findings) != 1 {
		t.Fatalf("res=%s err=%v", res, err)
	}
	if n := len([]rune(res.Findings[0].Summary)); n != 280 {
		t.Fatalf("summary runes = %d", n)
	}
}

func TestDuplicateAndMissingToolCallIDs(t *testing.T) {
	aud := &memAuditor{}
	prov := fake.New([]fake.Step{
		fake.ToolCalls(getCall("same", "http://lab/a"), getCall("same", "http://lab/b")),
		fake.Text("done"),
	}, fake.Options{})
	res, err := Run(context.Background(), params(t, aud, nil, prov, &denyPolicy{}, okVerifier{}))
	if err != nil {
		t.Fatal(err)
	}
	results := prov.Calls()[1].Messages[2].ToolResults
	if len(results) != 2 || results[0].IsError || !results[1].IsError || !strings.Contains(results[1].Content, "duplicate id") {
		t.Fatalf("results = %+v", results)
	}
	if res.ToolCalls != 2 {
		t.Fatalf("tool calls = %d", res.ToolCalls)
	}

	prov = fake.New([]fake.Step{fake.ToolCalls(getCall("x", "http://lab/a")), fake.Text("done")}, fake.Options{MissingToolCallIDs: true})
	_, err = Run(context.Background(), params(t, aud, nil, prov, &denyPolicy{}, okVerifier{}))
	if err != nil {
		t.Fatal(err)
	}
	results = prov.Calls()[1].Messages[2].ToolResults
	if len(results) != 1 || !results[0].IsError || !strings.Contains(results[0].Content, "missing id") {
		t.Fatalf("results = %+v", results)
	}
	// Rejected calls are still audited so the reconciler can see them.
	var rejected int
	for _, r := range aud.kinds("tool_call") {
		if r.Meta["is_error"] == true {
			rejected++
		}
	}
	if rejected != 2 {
		t.Fatalf("rejected tool_call records = %d", rejected)
	}
}

func TestUnknownToolName(t *testing.T) {
	prov := fake.New([]fake.Step{fake.ToolCalls(model.ToolCall{ID: "u1", Name: "shell", Args: map[string]any{"cmd": "id"}}), fake.Text("done")}, fake.Options{})
	pol := &denyPolicy{}
	_, err := Run(context.Background(), params(t, &memAuditor{}, nil, prov, pol, okVerifier{}))
	if err != nil {
		t.Fatal(err)
	}
	r := prov.Calls()[1].Messages[2].ToolResults[0]
	if !r.IsError || !strings.Contains(r.Content, "unknown tool") {
		t.Fatalf("result = %+v", r)
	}
	if len(pol.seen) != 0 {
		t.Fatal("policy evaluated for a tool with no manifest")
	}
}

func TestMaxTurns(t *testing.T) {
	steps := make([]fake.Step, 10)
	for i := range steps {
		steps[i] = fake.ToolCalls(getCall("c", "http://lab/a"))
	}
	prov := fake.New(steps, fake.Options{})
	p := params(t, &memAuditor{}, nil, prov, &denyPolicy{}, okVerifier{})
	p.MaxTurns = 3
	res, err := Run(context.Background(), p)
	if err != nil || res.Turns != 3 || res.StopReason != "max_turns" {
		t.Fatalf("res=%s err=%v", res, err)
	}
}

func TestRouterErrorPropagates(t *testing.T) {
	prov := fake.New([]fake.Step{fake.Fail(&provider.StatusError{Code: 401})}, fake.Options{})
	res, err := Run(context.Background(), params(t, &memAuditor{}, nil, prov, &denyPolicy{}, okVerifier{}))
	var se *provider.StatusError
	if !errors.As(err, &se) || res.StopReason != "error" || res.Turns != 0 {
		t.Fatalf("res=%s err=%v", res, err)
	}
}

func TestDuplicateFindingsMerged(t *testing.T) {
	prov := fake.New([]fake.Step{fake.ToolCalls(reportCall("r1"), reportCall("r2")), fake.Text("done")}, fake.Options{})
	res, err := Run(context.Background(), params(t, &memAuditor{}, nil, prov, &denyPolicy{}, okVerifier{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 || len(res.Findings[0].ControlPlane) != 2 {
		t.Fatalf("findings = %+v", res.Findings)
	}
}

func TestParamValidation(t *testing.T) {
	if _, err := Run(context.Background(), Params{}); !errors.Is(err, ErrNoRouter) {
		t.Fatalf("err = %v", err)
	}
	if _, err := Run(context.Background(), Params{Router: newRouter(nil, fake.New(nil, fake.Options{}))}); !errors.Is(err, ErrNoObjective) {
		t.Fatalf("err = %v", err)
	}
}

// --- tool call id reuse (ADR 0014) ---

// controlIDs returns the ToolCallID of every control-plane tool_call record
// plus the claimed id each carries (absent when the record's id is the
// claimed id).
func controlIDs(aud *memAuditor) (ids []string, claimed map[string]string) {
	claimed = map[string]string{}
	for _, rec := range aud.kinds("tool_call") {
		ids = append(ids, rec.ToolCallID)
		if c, ok := rec.Meta[MetaClaimedToolCallID].(string); ok {
			claimed[rec.ToolCallID] = c
		}
	}
	return ids, claimed
}

func TestReusedIDAcrossTurnsExecutesUnderControlID(t *testing.T) {
	aud := &memAuditor{}
	prov := fake.New([]fake.Step{
		fake.ToolCalls(getCall("c1", "http://lab/api/users/42")),
		fake.ToolCalls(getCall("c1", "http://lab/api/users/43")), // same id, new turn
		fake.ToolCalls(reportCall("c3")),                         // cites c1
		fake.Text("done"),
	}, fake.Options{})
	res, err := Run(context.Background(), params(t, aud, nil, prov, &denyPolicy{}, okVerifier{}))
	if err != nil {
		t.Fatal(err)
	}
	if res.ToolCalls != 3 || res.IDCollisions != 1 || res.Denied != 0 {
		t.Fatalf("result = %s", res)
	}
	// Both calls executed and the model saw its own id on both results.
	for turn := 1; turn <= 2; turn++ {
		results := prov.Calls()[turn].Messages[2*turn].ToolResults
		if len(results) != 1 || results[0].IsError || results[0].ToolCallID != "c1" {
			t.Fatalf("turn %d results = %+v", turn, results)
		}
	}
	ids, claimed := controlIDs(aud)
	if len(ids) != 3 || ids[0] != "c1" || ids[1] != "c1#2" || ids[2] != "c3" {
		t.Fatalf("control ids = %v", ids)
	}
	if claimed["c1#2"] != "c1" || len(claimed) != 1 {
		t.Fatalf("claimed meta = %v", claimed)
	}
	// Policy records follow the same control id.
	var policyIDs []string
	for _, rec := range aud.kinds("policy_decision") {
		policyIDs = append(policyIDs, rec.ToolCallID)
	}
	if len(policyIDs) != 3 || policyIDs[1] != "c1#2" {
		t.Fatalf("policy ids = %v", policyIDs)
	}
	// The finding keeps the model's claimed evidence id; the control-plane
	// reference points at the report_finding record.
	if len(res.Findings) != 1 || len(res.Findings[0].ToolCallIDs) != 1 || res.Findings[0].ToolCallIDs[0] != "c1" {
		t.Fatalf("findings = %+v", res.Findings)
	}
	if !strings.Contains(res.String(), "id_collisions=1") {
		t.Fatalf("String() = %s", res)
	}
}

func TestDuplicateToolCallIDsOptionRecordsEveryCallUniquely(t *testing.T) {
	aud := &memAuditor{}
	// DuplicateToolCallIDs gives every call in a response the first call's
	// id, in both turns: the id is reused within a turn and across turns.
	prov := fake.New([]fake.Step{
		fake.ToolCalls(getCall("dup", "http://lab/a"), getCall("other", "http://lab/b")),
		fake.ToolCalls(getCall("dup", "http://lab/c"), getCall("other2", "http://lab/d")),
		fake.Text("done"),
	}, fake.Options{DuplicateToolCallIDs: true})
	res, err := Run(context.Background(), params(t, aud, nil, prov, &denyPolicy{}, okVerifier{}))
	if err != nil {
		t.Fatal(err)
	}
	if res.ToolCalls != 4 || res.IDCollisions != 3 {
		t.Fatalf("result = %s", res)
	}
	// Within a turn the second call is rejected; across turns it executes.
	r1 := prov.Calls()[1].Messages[2].ToolResults
	r2 := prov.Calls()[2].Messages[4].ToolResults
	if len(r1) != 2 || r1[0].IsError || !r1[1].IsError || !strings.Contains(r1[1].Content, "duplicate id") {
		t.Fatalf("turn 1 results = %+v", r1)
	}
	if len(r2) != 2 || r2[0].IsError || !r2[1].IsError {
		t.Fatalf("turn 2 results = %+v", r2)
	}
	for _, rs := range [][]model.ToolResult{r1, r2} {
		for _, r := range rs {
			if r.ToolCallID != "dup" {
				t.Fatalf("model must see its own id, got %q", r.ToolCallID)
			}
		}
	}
	ids, claimed := controlIDs(aud)
	want := []string{"dup", "dup#1", "dup#2", "dup#2.2"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("control ids = %v, want %v", ids, want)
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("control id %q recorded twice", id)
		}
		seen[id] = true
	}
	for _, id := range want[1:] {
		if claimed[id] != "dup" {
			t.Fatalf("%s claimed = %q", id, claimed[id])
		}
	}
	if _, ok := claimed["dup"]; ok {
		t.Fatal("first use must not carry a claimed_tool_call_id")
	}
}

func TestSharedIDSpaceDisambiguatesAcrossTasks(t *testing.T) {
	script := func() *fake.Provider {
		return fake.New([]fake.Step{fake.ToolCalls(getCall("fk-1", "http://lab/")), fake.Text("done")}, fake.Options{})
	}
	// Separate spaces (the default): both tasks record fk-1, which is what
	// the committed run shows for one script run against several labs.
	aud := &memAuditor{}
	for _, task := range []string{"lab-a:m", "lab-b:m"} {
		p := params(t, aud, nil, script(), &denyPolicy{}, okVerifier{})
		p.TaskID = task
		if _, err := Run(context.Background(), p); err != nil {
			t.Fatal(err)
		}
	}
	ids, _ := controlIDs(aud)
	if strings.Join(ids, ",") != "fk-1,fk-1" {
		t.Fatalf("per-task spaces: %v", ids)
	}
	// One space shared by every agent of the run: the second task's call
	// is recorded under a derived id and names the claimed one.
	aud = &memAuditor{}
	space := NewIDSpace()
	for _, task := range []string{"lab-a:m", "lab-b:m"} {
		p := params(t, aud, nil, script(), &denyPolicy{}, okVerifier{})
		p.TaskID, p.IDs = task, space
		res, err := Run(context.Background(), p)
		if err != nil {
			t.Fatal(err)
		}
		if task == "lab-b:m" && res.IDCollisions != 1 {
			t.Fatalf("second task collisions = %d", res.IDCollisions)
		}
	}
	ids, claimed := controlIDs(aud)
	if strings.Join(ids, ",") != "fk-1,fk-1#1" || claimed["fk-1#1"] != "fk-1" {
		t.Fatalf("shared space: %v %v", ids, claimed)
	}
}

func TestIDSpaceControlIsUniqueAndDeterministic(t *testing.T) {
	s := NewIDSpace()
	cases := []struct {
		claimed string
		turn    int
		want    string
		collide bool
	}{
		{"a", 1, "a", false},
		{"a", 1, "a#1", true},
		{"a", 1, "a#1.2", true},
		{"a#2", 1, "a#2", false}, // a model may claim the derived shape itself
		{"a", 2, "a#2.2", true},
		{"b", 2, "b", false},
	}
	for _, c := range cases {
		got, collided := s.control(c.claimed, c.turn)
		if got != c.want || collided != c.collide {
			t.Fatalf("control(%q,%d) = %q,%v want %q,%v", c.claimed, c.turn, got, collided, c.want, c.collide)
		}
	}
	var zero IDSpace // usable without the constructor
	if got, _ := zero.control("z", 1); got != "z" {
		t.Fatalf("zero value: %q", got)
	}
}
