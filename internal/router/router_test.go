package router

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/provider"
	"github.com/joeyvictorino/assay/internal/provider/fake"
)

type memAuditor struct {
	mu   sync.Mutex
	recs []model.AuditRecord
}

func (a *memAuditor) Record(_ context.Context, rec model.AuditRecord) (uint64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.recs = append(a.recs, rec)
	return uint64(len(a.recs)), nil
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

var costs = provider.Costs{
	"claude-sonnet-5-5": {InputUSD: 2, OutputUSD: 10, CacheReadUSD: 0.20},
	"claude-haiku-4-5":  {InputUSD: 1, OutputUSD: 5, CacheReadUSD: 0.10},
	"local":             {},
}

func baseConfig() Config {
	return Config{
		Routes: map[string][]model.ModelRef{
			"probe": {
				{Provider: "a", Model: "claude-sonnet-5-5", Agent: "probe"},
				{Provider: "b", Model: "claude-haiku-4-5", Agent: "probe"},
			},
		},
		Budgets: Budgets{
			PerRun:   model.Budget{MaxUSD: 15},
			PerModel: model.Budget{MaxUSD: 6},
			PerTask:  model.Budget{MaxUSD: 1.5, MaxLatencyMS: 5000},
		},
		Costs: costs,
		RunID: "run-test",
	}
}

func registry(a, b model.Provider) *provider.Registry {
	reg := provider.NewRegistry()
	if a != nil {
		reg.Register("a", a)
	}
	if b != nil {
		reg.Register("b", b)
	}
	return reg
}

func req() model.Request {
	return model.Request{System: "s", Messages: []model.Message{{Role: "user", Content: "go"}}, TaskID: "t1", Agent: "probe"}
}

func TestRouterHappyPath(t *testing.T) {
	aud := &memAuditor{}
	pa := fake.New([]fake.Step{fake.ToolCalls(model.ToolCall{ID: "tc1", Name: "http_get", Args: map[string]any{"url": "u"}})}, Options0())
	pb := fake.New([]fake.Step{fake.Text("b")}, fake.Options{})
	r := New(baseConfig(), registry(pa, pb), aud)
	resp, dec, err := r.Complete(context.Background(), "probe", req())
	if err != nil {
		t.Fatal(err)
	}
	if dec.Chosen != "a/claude-sonnet-5-5" || dec.Reason != "first_candidate_with_budget" || dec.Failovers != 0 {
		t.Fatalf("decision = %+v", dec)
	}
	if len(dec.Candidates) != 2 {
		t.Fatalf("candidates = %v", dec.Candidates)
	}
	// 120 in @ $2/M + 40 out @ $10/M
	want := 120*2/1e6 + 40*10/1e6
	if math.Abs(resp.Usage.CostUSD-want) > 1e-12 || math.Abs(r.Spent()-want) > 1e-12 {
		t.Fatalf("cost = %v spent = %v want %v", resp.Usage.CostUSD, r.Spent(), want)
	}
	if math.Abs(dec.BudgetRemainingUSD-(15-want)) > 1e-9 {
		t.Fatalf("remaining = %v", dec.BudgetRemainingUSD)
	}
	if pa.Calls()[0].Model != "claude-sonnet-5-5" {
		t.Fatalf("request model not set: %q", pa.Calls()[0].Model)
	}
	if pb.Remaining() != 1 {
		t.Fatal("second candidate must not be called")
	}

	calls := aud.kinds("model_call")
	if len(calls) != 1 {
		t.Fatalf("model_call records = %d", len(calls))
	}
	mc := calls[0]
	if mc.Meta["model"] != "claude-sonnet-5-5" || mc.Meta["provider"] != "a" || mc.Meta["declined"] != false {
		t.Fatalf("meta = %v", mc.Meta)
	}
	if mc.Meta["cost_microusd"].(int64) != provider.MicroUSD(want) {
		t.Fatalf("cost_microusd = %v", mc.Meta["cost_microusd"])
	}
	if len(mc.Hashes["prompt"]) != 64 || len(mc.Hashes["response"]) != 64 {
		t.Fatalf("hashes = %v", mc.Hashes)
	}
	claimed := mc.Meta["claimed_tool_calls"].([]map[string]any)
	if len(claimed) != 1 || claimed[0]["id"] != "tc1" || claimed[0]["name"] != "http_get" || len(claimed[0]["args_digest"].(string)) != 64 {
		t.Fatalf("claimed = %v", claimed)
	}
	for k, v := range mc.Meta {
		if s, ok := v.(string); ok && strings.Contains(s, "go") && k != "stop_reason" {
			t.Fatalf("prompt body leaked into meta %s=%q", k, s)
		}
	}
	decs := aud.kinds("router_decision")
	if len(decs) != 1 || decs[0].Meta["chosen"] != "a/claude-sonnet-5-5" || decs[0].Meta["task_kind"] != "probe" {
		t.Fatalf("router_decision = %v", decs)
	}
	if _, ok := decs[0].Meta["budget_remaining_usd"]; !ok {
		t.Fatal("budget_remaining_usd missing")
	}
	if mc.RunID != "run-test" || mc.TaskID != "t1" || mc.Agent != "probe" {
		t.Fatalf("record ids = %+v", mc)
	}
}

// Options0 returns zero fake options; it exists so the happy-path test reads
// symmetrically with the fault-injection tests below.
func Options0() fake.Options { return fake.Options{} }

func TestRouterFailoverOnRetryable(t *testing.T) {
	aud := &memAuditor{}
	pa := fake.New([]fake.Step{fake.Fail(&provider.StatusError{Provider: "a", Code: 429})}, fake.Options{})
	pb := fake.New([]fake.Step{fake.Text("from b")}, fake.Options{})
	r := New(baseConfig(), registry(pa, pb), aud)
	resp, dec, err := r.Complete(context.Background(), "probe", req())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "from b" || dec.Chosen != "b/claude-haiku-4-5" || dec.Failovers != 1 {
		t.Fatalf("resp=%+v dec=%+v", resp, dec)
	}
	if !strings.HasPrefix(dec.Reason, "failover_from:") || !strings.Contains(dec.Reason, "retryable_error") {
		t.Fatalf("reason = %q", dec.Reason)
	}
	calls := aud.kinds("model_call")
	if len(calls) != 2 {
		t.Fatalf("model_call records = %d (failed attempt must be audited)", len(calls))
	}
	if calls[0].Meta["error"] == nil || calls[0].Meta["retryable"] != true {
		t.Fatalf("failed call meta = %v", calls[0].Meta)
	}
}

func TestRouterNoFailoverOnNonRetryable(t *testing.T) {
	pa := fake.New([]fake.Step{fake.Fail(&provider.StatusError{Provider: "a", Code: 400})}, fake.Options{})
	pb := fake.New([]fake.Step{fake.Text("from b")}, fake.Options{})
	r := New(baseConfig(), registry(pa, pb), &memAuditor{})
	_, dec, err := r.Complete(context.Background(), "probe", req())
	var se *provider.StatusError
	if !errors.As(err, &se) || se.Code != 400 {
		t.Fatalf("err = %v", err)
	}
	if dec.Failovers != 0 || pb.Remaining() != 1 {
		t.Fatalf("failed over on 400: dec=%+v", dec)
	}
	if !strings.HasPrefix(dec.Reason, "error:") {
		t.Fatalf("reason = %q", dec.Reason)
	}
}

func TestRouterMaxOneFailover(t *testing.T) {
	cfg := baseConfig()
	cfg.Routes["probe"] = append(cfg.Routes["probe"], model.ModelRef{Provider: "c", Model: "local"})
	pa := fake.New([]fake.Step{fake.Fail(&provider.StatusError{Code: 503})}, fake.Options{})
	pb := fake.New([]fake.Step{fake.Fail(&provider.StatusError{Code: 503})}, fake.Options{})
	pc := fake.New([]fake.Step{fake.Text("c")}, fake.Options{})
	reg := registry(pa, pb)
	reg.Register("c", pc)
	r := New(cfg, reg, nil)
	_, dec, err := r.Complete(context.Background(), "probe", req())
	if err == nil {
		t.Fatal("expected error after one failover")
	}
	if dec.Failovers != 1 || pc.Remaining() != 1 {
		t.Fatalf("dec=%+v third called=%v", dec, pc.Remaining() == 0)
	}

	// Configurable: allow two failovers and the third candidate answers.
	cfg.MaxFailovers = 2
	pa = fake.New([]fake.Step{fake.Fail(&provider.StatusError{Code: 503})}, fake.Options{})
	pb = fake.New([]fake.Step{fake.Fail(&provider.StatusError{Code: 503})}, fake.Options{})
	reg = registry(pa, pb)
	reg.Register("c", pc)
	r = New(cfg, reg, nil)
	resp, dec, err := r.Complete(context.Background(), "probe", req())
	if err != nil || resp.Text != "c" || dec.Failovers != 2 {
		t.Fatalf("resp=%+v dec=%+v err=%v", resp, dec, err)
	}

	// Disabled: negative MaxFailovers.
	cfg.MaxFailovers = -1
	pa = fake.New([]fake.Step{fake.Fail(&provider.StatusError{Code: 503})}, fake.Options{})
	pb = fake.New([]fake.Step{fake.Text("b")}, fake.Options{})
	r = New(cfg, registry(pa, pb), nil)
	if _, dec, err := r.Complete(context.Background(), "probe", req()); err == nil || dec.Failovers != 0 {
		t.Fatalf("failover not disabled: dec=%+v err=%v", dec, err)
	}
}

func TestRouterNeverRetriesDeclined(t *testing.T) {
	aud := &memAuditor{}
	pa := fake.New([]fake.Step{fake.Refusal(), fake.Text("second try")}, fake.Options{})
	pb := fake.New([]fake.Step{fake.Text("from b")}, fake.Options{})
	r := New(baseConfig(), registry(pa, pb), aud)
	resp, dec, err := r.Complete(context.Background(), "probe", req())
	if err != nil {
		t.Fatalf("declined must not be an error: %v", err)
	}
	if !resp.Declined || resp.StopReason != "refusal" {
		t.Fatalf("resp = %+v", resp)
	}
	if dec.Failovers != 0 || dec.Chosen != "a/claude-sonnet-5-5" || !strings.HasPrefix(dec.Reason, "declined:") {
		t.Fatalf("dec = %+v", dec)
	}
	if pa.Remaining() != 1 {
		t.Fatal("router retried the declining model")
	}
	if pb.Remaining() != 1 {
		t.Fatal("router failed over after a refusal")
	}
	calls := aud.kinds("model_call")
	if len(calls) != 1 || calls[0].Meta["declined"] != true {
		t.Fatalf("model_call = %v", calls)
	}
	if r.Spent() == 0 {
		t.Fatal("refusal tokens must still be charged")
	}
}

func TestRouterPerModelBudgetSkips(t *testing.T) {
	cfg := baseConfig()
	cfg.Budgets.PerModel.MaxUSD = 0.0001
	cfg.Budgets.PerTask.MaxUSD = 0 // uncapped
	// First call costs 0.0012 > 0.0001, so the second call must skip model a.
	pa := fake.New([]fake.Step{fake.Text("a1"), fake.Text("a2")}, fake.Options{})
	pb := fake.New([]fake.Step{fake.Text("b1")}, fake.Options{})
	r := New(cfg, registry(pa, pb), nil)
	if _, dec, err := r.Complete(context.Background(), "probe", req()); err != nil || dec.Chosen != "a/claude-sonnet-5-5" {
		t.Fatalf("first: %+v %v", dec, err)
	}
	resp, dec, err := r.Complete(context.Background(), "probe", req())
	if err != nil || resp.Text != "b1" {
		t.Fatalf("second: %+v %v", resp, err)
	}
	if dec.Chosen != "b/claude-haiku-4-5" || !strings.Contains(dec.Reason, "a/claude-sonnet-5-5:model_budget") || dec.Failovers != 0 {
		t.Fatalf("dec = %+v", dec)
	}
	// Third: both models over budget.
	_, dec, err = r.Complete(context.Background(), "probe", req())
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("err = %v", err)
	}
	if dec.Chosen != "" || !strings.HasPrefix(dec.Reason, "budget_exceeded:") {
		t.Fatalf("dec = %+v", dec)
	}
	by := r.SpentByModel()
	if by["claude-sonnet-5-5"] == 0 || by["claude-haiku-4-5"] == 0 {
		t.Fatalf("spend by model = %v", by)
	}
}

func TestRouterPerRunBudget(t *testing.T) {
	cfg := baseConfig()
	cfg.Budgets.PerRun.MaxUSD = 0.0003 // one Text step costs 0.0004
	pa := fake.New([]fake.Step{fake.Text("a1"), fake.Text("a2")}, fake.Options{})
	pb := fake.New([]fake.Step{fake.Text("b1")}, fake.Options{})
	r := New(cfg, registry(pa, pb), nil)
	if _, _, err := r.Complete(context.Background(), "probe", req()); err != nil {
		t.Fatal(err)
	}
	_, dec, err := r.Complete(context.Background(), "probe", req())
	if !errors.Is(err, ErrBudgetExceeded) || !strings.Contains(dec.Reason, "run_budget") {
		t.Fatalf("err=%v dec=%+v", err, dec)
	}
	if pb.Remaining() != 1 {
		t.Fatal("run budget exceeded must not try other candidates")
	}
	if dec.BudgetRemainingUSD != 0 {
		t.Fatalf("remaining = %v", dec.BudgetRemainingUSD)
	}
}

func TestRouterPerTaskBudget(t *testing.T) {
	cfg := baseConfig()
	cfg.Budgets.PerTask.MaxUSD = 0.0003 // one Text step costs 0.0004
	pa := fake.New([]fake.Step{fake.Text("a1"), fake.Text("a2"), fake.Text("a3")}, fake.Options{})
	r := New(cfg, registry(pa, nil), nil)
	r1 := req()
	if _, _, err := r.Complete(context.Background(), "probe", r1); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Complete(context.Background(), "probe", r1); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("same task should be capped: %v", err)
	}
	r2 := req()
	r2.TaskID = "t2"
	if _, _, err := r.Complete(context.Background(), "probe", r2); err != nil {
		t.Fatalf("other task should proceed: %v", err)
	}
}

type slowProvider struct{ delay time.Duration }

func (s slowProvider) Name() string { return "slow" }
func (s slowProvider) Complete(ctx context.Context, _ model.Request) (model.Response, error) {
	select {
	case <-ctx.Done():
		return model.Response{}, ctx.Err()
	case <-time.After(s.delay):
		return model.Response{Text: "late", StopReason: "end_turn"}, nil
	}
}

func TestRouterLatencyDeadline(t *testing.T) {
	cfg := baseConfig()
	cfg.Budgets.PerTask.MaxLatencyMS = 20
	cfg.MaxFailovers = -1
	r := New(cfg, registry(slowProvider{delay: 2 * time.Second}, nil), nil)
	start := time.Now()
	_, _, err := r.Complete(context.Background(), "probe", req())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("deadline not enforced")
	}

	// A per-model override lengthens the deadline for a slow local model.
	cfg.ModelLatencyMS = map[string]int64{"claude-sonnet-5-5": 2000}
	r = New(cfg, registry(slowProvider{delay: 50 * time.Millisecond}, nil), nil)
	if resp, _, err := r.Complete(context.Background(), "probe", req()); err != nil || resp.Text != "late" {
		t.Fatalf("override: %+v %v", resp, err)
	}
	if resp, _, _ := r.Complete(context.Background(), "probe", req()); resp.Usage.LatencyMS <= 0 {
		t.Fatalf("latency not measured: %+v", resp.Usage)
	}
}

func TestRouterDeadlineFailsOver(t *testing.T) {
	cfg := baseConfig()
	cfg.Budgets.PerTask.MaxLatencyMS = 20
	pb := fake.New([]fake.Step{fake.Text("b")}, fake.Options{})
	r := New(cfg, registry(slowProvider{delay: time.Second}, pb), nil)
	resp, dec, err := r.Complete(context.Background(), "probe", req())
	if err != nil || resp.Text != "b" || dec.Failovers != 1 {
		t.Fatalf("resp=%+v dec=%+v err=%v", resp, dec, err)
	}
}

func TestRouterNoRouteUnknownProviderMissingCost(t *testing.T) {
	cfg := baseConfig()
	r := New(cfg, registry(fake.New(nil, fake.Options{}), nil), &memAuditor{})
	if _, dec, err := r.Complete(context.Background(), "nope", req()); !errors.Is(err, ErrNoRoute) || dec.Reason != "no_route" {
		t.Fatalf("no route: %v %+v", err, dec)
	}

	cfg.Routes["probe"] = []model.ModelRef{{Provider: "zzz", Model: "claude-sonnet-5-5"}}
	r = New(cfg, registry(nil, nil), nil)
	if _, dec, err := r.Complete(context.Background(), "probe", req()); !errors.Is(err, provider.ErrUnknownProvider) || !strings.Contains(dec.Reason, "unknown_provider") {
		t.Fatalf("unknown provider: %v %+v", err, dec)
	}

	cfg.Routes["probe"] = []model.ModelRef{{Provider: "a", Model: "unpriced"}}
	r = New(cfg, registry(fake.New([]fake.Step{fake.Text("x")}, fake.Options{}), nil), nil)
	if _, dec, err := r.Complete(context.Background(), "probe", req()); !errors.Is(err, provider.ErrMissingCost) || !strings.Contains(dec.Reason, "missing_cost") {
		t.Fatalf("missing cost: %v %+v", err, dec)
	}
}

func TestRouterConcurrentSpendIsExact(t *testing.T) {
	cfg := baseConfig()
	cfg.Budgets.PerTask.MaxUSD = 0
	const n = 200
	steps := make([]fake.Step, n)
	for i := range steps {
		steps[i] = fake.Text("x")
	}
	pa := fake.New(steps, fake.Options{})
	r := New(cfg, registry(pa, nil), &memAuditor{})
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = r.Complete(context.Background(), "probe", req())
		}()
	}
	wg.Wait()
	per := provider.ComputeCost(model.Usage{InputTokens: 100, OutputTokens: 20}, costs["claude-sonnet-5-5"])
	if math.Abs(r.Spent()-per*n) > 1e-9 {
		t.Fatalf("spent = %v want %v", r.Spent(), per*n)
	}
}

func TestPromptHashStable(t *testing.T) {
	a := PromptHash(req())
	b := PromptHash(req())
	if a != b || len(a) != 64 {
		t.Fatalf("hash unstable: %s %s", a, b)
	}
	other := req()
	other.Messages[0].Content = "different"
	if PromptHash(other) == a {
		t.Fatal("hash does not depend on content")
	}
}
