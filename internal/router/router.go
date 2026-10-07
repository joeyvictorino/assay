// Package router picks a model for each task kind, enforces USD and latency
// budgets, fails over once on transient transport errors, and records every
// decision and model call in the audit log. Refusals are returned as-is:
// never retried, never failed over, never rephrased.
package router

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/provider"
)

var (
	// ErrBudgetExceeded is returned when the per-run, per-model or per-task
	// USD cap leaves no candidate with room.
	ErrBudgetExceeded = errors.New("router: budget exceeded")
	// ErrNoRoute is returned for a task kind with no candidates.
	ErrNoRoute = errors.New("router: no route for task kind")
)

// Budgets groups the three cap scopes.
type Budgets struct {
	PerRun   model.Budget
	PerModel model.Budget
	PerTask  model.Budget
}

// Config configures a Router.
type Config struct {
	// Routes lists ordered candidates per task kind.
	Routes map[string][]model.ModelRef
	// Budgets holds the USD and latency caps. MaxUSD <= 0 means uncapped.
	Budgets Budgets
	// Costs prices every model that appears in Routes.
	Costs provider.Costs
	// ModelLatencyMS overrides Budgets.PerTask.MaxLatencyMS per model id
	// (a slow local model gets a longer deadline).
	ModelLatencyMS map[string]int64
	// MaxFailovers bounds failovers per Complete call. Zero means the
	// default of one; negative disables failover.
	MaxFailovers int
	// RunID is stamped on audit records.
	RunID string
}

// Decision explains one Complete call.
type Decision struct {
	TaskKind           string   `json:"task_kind"`
	Candidates         []string `json:"candidates"`
	Chosen             string   `json:"chosen"`
	Reason             string   `json:"reason"`
	BudgetRemainingUSD float64  `json:"budget_remaining_usd"`
	Failovers          int      `json:"failovers"`
}

// RouterDecision is the name the rest of the harness uses for Decision.
type RouterDecision = Decision

// Router implements the routing plane.
type Router struct {
	cfg       Config
	providers *provider.Registry
	auditor   model.Auditor
	now       func() time.Time

	mu       sync.Mutex
	runSpent float64
	byModel  map[string]float64
	byTask   map[string]float64
}

// New builds a Router. auditor may be nil in tests that do not care about
// records; in production it is always set.
func New(cfg Config, providers *provider.Registry, auditor model.Auditor) *Router {
	if cfg.MaxFailovers == 0 {
		cfg.MaxFailovers = 1
	}
	return &Router{
		cfg:       cfg,
		providers: providers,
		auditor:   auditor,
		now:       time.Now,
		byModel:   map[string]float64{},
		byTask:    map[string]float64{},
	}
}

// Spent reports the USD spent so far in the run.
func (r *Router) Spent() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.runSpent
}

// SpentByModel reports per-model spend, keyed by model id.
func (r *Router) SpentByModel() map[string]float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]float64, len(r.byModel))
	for k, v := range r.byModel {
		out[k] = v
	}
	return out
}

func refKey(m model.ModelRef) string { return m.Provider + "/" + m.Model }

func capped(max float64) bool { return max > 0 }

// Complete routes one request. req.Model is overwritten with the chosen
// candidate's model id. The returned Decision is populated even on error.
func (r *Router) Complete(ctx context.Context, taskKind string, req model.Request) (model.Response, Decision, error) {
	dec := Decision{TaskKind: taskKind}
	candidates := r.cfg.Routes[taskKind]
	if len(candidates) == 0 {
		dec.Reason = "no_route"
		r.auditDecision(ctx, req, dec)
		return model.Response{}, dec, fmt.Errorf("%w: %q", ErrNoRoute, taskKind)
	}
	for _, c := range candidates {
		dec.Candidates = append(dec.Candidates, refKey(c))
	}

	var skipped []string
	var lastErr error
	for i, cand := range candidates {
		if reason := r.roomFor(cand, req.TaskID); reason != "" {
			skipped = append(skipped, refKey(cand)+":"+reason)
			if strings.HasPrefix(reason, "run_budget") || strings.HasPrefix(reason, "task_budget") {
				break // no candidate can help
			}
			continue
		}
		prov, err := r.providers.Get(cand.Provider)
		if err != nil {
			skipped = append(skipped, refKey(cand)+":unknown_provider")
			lastErr = err
			continue
		}
		cost, err := r.cfg.Costs.Lookup(cand.Model)
		if err != nil {
			skipped = append(skipped, refKey(cand)+":missing_cost")
			lastErr = err
			continue
		}

		dec.Chosen = refKey(cand)
		if dec.Failovers == 0 {
			dec.Reason = "first_candidate_with_budget"
		} else {
			dec.Reason = "failover_from:" + strings.Join(skipped, ",")
		}
		if len(skipped) > 0 && dec.Failovers == 0 {
			dec.Reason = "skipped:" + strings.Join(skipped, ",")
		}

		creq := req
		creq.Model = cand.Model
		if creq.Agent == "" {
			creq.Agent = cand.Agent
		}
		resp, err := r.call(ctx, prov, cand, creq)
		if err != nil {
			lastErr = err
			r.auditCall(ctx, cand, creq, model.Response{StopReason: "error"}, 0, err)
			if provider.Classify(err) && dec.Failovers < r.cfg.MaxFailovers && i+1 < len(candidates) {
				dec.Failovers++
				skipped = append(skipped, refKey(cand)+":retryable_error")
				continue
			}
			dec.Reason = "error:" + refKey(cand)
			dec.BudgetRemainingUSD = r.remaining()
			r.auditDecision(ctx, req, dec)
			return model.Response{}, dec, err
		}

		usd := provider.ComputeCost(resp.Usage, cost)
		resp.Usage.CostUSD = usd
		r.addSpend(cand.Model, req.TaskID, usd)
		r.auditCall(ctx, cand, creq, resp, usd, nil)
		dec.BudgetRemainingUSD = r.remaining()
		if resp.Declined {
			dec.Reason = "declined:" + refKey(cand)
			r.auditDecision(ctx, req, dec)
			return resp, dec, nil
		}
		r.auditDecision(ctx, req, dec)
		return resp, dec, nil
	}

	dec.Chosen = ""
	dec.BudgetRemainingUSD = r.remaining()
	if lastErr != nil {
		dec.Reason = "exhausted:" + strings.Join(skipped, ",")
		r.auditDecision(ctx, req, dec)
		return model.Response{}, dec, lastErr
	}
	dec.Reason = "budget_exceeded:" + strings.Join(skipped, ",")
	r.auditDecision(ctx, req, dec)
	return model.Response{}, dec, fmt.Errorf("%w: %s", ErrBudgetExceeded, strings.Join(skipped, ","))
}

// roomFor returns "" when cand may be called, else the reason it may not.
func (r *Router) roomFor(cand model.ModelRef, taskID string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	b := r.cfg.Budgets
	if capped(b.PerRun.MaxUSD) && r.runSpent >= b.PerRun.MaxUSD {
		return "run_budget"
	}
	if capped(b.PerTask.MaxUSD) && taskID != "" && r.byTask[taskID] >= b.PerTask.MaxUSD {
		return "task_budget"
	}
	if capped(b.PerModel.MaxUSD) && r.byModel[cand.Model] >= b.PerModel.MaxUSD {
		return "model_budget"
	}
	return ""
}

func (r *Router) addSpend(modelID, taskID string, usd float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runSpent += usd
	r.byModel[modelID] += usd
	if taskID != "" {
		r.byTask[taskID] += usd
	}
}

func (r *Router) remaining() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !capped(r.cfg.Budgets.PerRun.MaxUSD) {
		return 0
	}
	rem := r.cfg.Budgets.PerRun.MaxUSD - r.runSpent
	if rem < 0 {
		rem = 0
	}
	return rem
}

func (r *Router) deadlineFor(modelID string) time.Duration {
	ms := r.cfg.Budgets.PerTask.MaxLatencyMS
	if v, ok := r.cfg.ModelLatencyMS[modelID]; ok && v > 0 {
		ms = v
	}
	return time.Duration(ms) * time.Millisecond
}

func (r *Router) call(ctx context.Context, prov model.Provider, cand model.ModelRef, req model.Request) (model.Response, error) {
	if d := r.deadlineFor(cand.Model); d > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d)
		defer cancel()
	}
	start := r.now()
	resp, err := prov.Complete(ctx, req)
	if err != nil {
		return model.Response{}, err
	}
	if resp.Usage.LatencyMS == 0 {
		resp.Usage.LatencyMS = r.now().Sub(start).Milliseconds()
	}
	if resp.Declined {
		resp.StopReason = "refusal"
	}
	return resp, nil
}

// PromptHash is sha256 over the JSON encoding of req. encoding/json emits
// struct fields in declaration order and map keys sorted, so the digest is
// stable for equal requests.
func PromptHash(req model.Request) string {
	b, _ := json.Marshal(req)
	return digest(b)
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func argsDigest(args map[string]any) string {
	if args == nil {
		args = map[string]any{}
	}
	b, _ := json.Marshal(args)
	return digest(b)
}

func (r *Router) auditDecision(ctx context.Context, req model.Request, dec Decision) {
	if r.auditor == nil {
		return
	}
	cands := append([]string(nil), dec.Candidates...)
	if cands == nil {
		cands = []string{}
	}
	_, _ = r.auditor.Record(ctx, model.AuditRecord{
		Time:   r.now().UTC(),
		RunID:  r.cfg.RunID,
		TaskID: req.TaskID,
		Agent:  req.Agent,
		Kind:   "router_decision",
		Meta: map[string]any{
			"task_kind":            dec.TaskKind,
			"candidates":           cands,
			"chosen":               dec.Chosen,
			"reason":               dec.Reason,
			"budget_remaining_usd": dec.BudgetRemainingUSD,
			"failovers":            dec.Failovers,
		},
	})
}

func (r *Router) auditCall(ctx context.Context, cand model.ModelRef, req model.Request, resp model.Response, usd float64, callErr error) {
	if r.auditor == nil {
		return
	}
	claimed := make([]map[string]any, 0, len(resp.ToolCalls))
	for _, tc := range resp.ToolCalls {
		claimed = append(claimed, map[string]any{"id": tc.ID, "name": tc.Name, "args_digest": argsDigest(tc.Args)})
	}
	sort.Slice(claimed, func(i, j int) bool { return claimed[i]["id"].(string) < claimed[j]["id"].(string) })
	meta := map[string]any{
		"model":              cand.Model,
		"provider":           cand.Provider,
		"tokens":             resp.Usage.InputTokens + resp.Usage.OutputTokens,
		"input_tokens":       resp.Usage.InputTokens,
		"output_tokens":      resp.Usage.OutputTokens,
		"cache_read":         resp.Usage.CacheReadTokens,
		"cache_write":        resp.Usage.CacheWriteTokens,
		"latency_ms":         resp.Usage.LatencyMS,
		"cost_microusd":      provider.MicroUSD(usd),
		"declined":           resp.Declined,
		"stop_reason":        resp.StopReason,
		"claimed_tool_calls": claimed,
	}
	if resp.RequestID != "" {
		meta["request_id"] = resp.RequestID
	}
	if callErr != nil {
		meta["error"] = callErr.Error()
		meta["retryable"] = provider.Classify(callErr)
	}
	_, _ = r.auditor.Record(ctx, model.AuditRecord{
		Time:   r.now().UTC(),
		RunID:  r.cfg.RunID,
		TaskID: req.TaskID,
		Agent:  req.Agent,
		Kind:   "model_call",
		Hashes: map[string]string{
			"prompt":   PromptHash(req),
			"response": digest([]byte(resp.Text)),
		},
		Meta: meta,
	})
}
