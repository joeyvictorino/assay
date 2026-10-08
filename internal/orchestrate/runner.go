package orchestrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/policy"
)

// Task statuses in a TaskResult.
const (
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusSkipped   = "skipped"
)

// Attempt kinds recorded in TaskResult.Attempts.
const (
	AttemptExec     = "exec"     // first execution
	AttemptRetry    = "retry"    // re-execution after an exec error
	AttemptRevision = "revision" // revision after a low score
)

// Reflection bounds.
const (
	// PassScore is the lowest score that needs no revision.
	PassScore = 7
	// MaxRevisions bounds the number of revisions per task.
	MaxRevisions = 2
	// MaxRetries bounds the number of re-executions after an exec error.
	MaxRetries = 1
)

// Attempt is one execution, retry or revision of a task.
type Attempt struct {
	N        int    `json:"n"`
	Kind     string `json:"kind"`
	Err      string `json:"err,omitempty"`
	Scored   bool   `json:"scored"`
	Score    int    `json:"score,omitempty"`
	Critique string `json:"critique,omitempty"`
}

// TaskResult is the outcome of one task. Output is opaque to the runner;
// exec, score and revise functions agree on its meaning.
type TaskResult struct {
	TaskID   string    `json:"task_id"`
	Status   string    `json:"status"` // succeeded | failed | skipped
	Reason   string    `json:"reason,omitempty"`
	Output   string    `json:"output,omitempty"`
	Score    int       `json:"score,omitempty"`
	Attempts []Attempt `json:"attempts,omitempty"`
}

// PlanResult is the outcome of a plan run. Tasks are in schedule order.
type PlanResult struct {
	Tasks     []TaskResult `json:"tasks"`
	Waves     int          `json:"waves"`
	Succeeded int          `json:"succeeded"`
	Failed    int          `json:"failed"`
	Skipped   int          `json:"skipped"`
	Err       string       `json:"err,omitempty"` // plan validation error, if any
}

// Exec runs one task.
type Exec func(ctx context.Context, task model.Task) (TaskResult, error)

// Score grades a task result from 0 to 10 with a critique.
type Score func(ctx context.Context, task model.Task, res TaskResult) (int, string, error)

// Revise produces a new result for a task given the critique of the last.
type Revise func(ctx context.Context, task model.Task, critique string) (TaskResult, error)

// Runner executes plans. Score and Revise are optional; without Score no
// reflection happens, without Revise a low score is recorded but not acted
// on. Auditor is optional; when set every score and delegation decision is
// recorded as digests and small metadata.
type Runner struct {
	Score   Score
	Revise  Revise
	Auditor model.Auditor
	RunID   string
	// Now supplies timestamps; nil means time.Now.
	Now func() time.Time

	mu      sync.Mutex
	lineage map[string][]string // task id -> agent ids above it
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// Run validates the plan, then executes it wave by wave. Tasks within a
// wave run concurrently; exec, score and revise must be safe for that. A
// task whose dependency did not succeed is skipped with a reason naming
// that dependency. A plan that fails validation produces a PlanResult with
// Err set and no tasks.
func (r *Runner) Run(ctx context.Context, plan Plan, exec Exec) PlanResult {
	waves, err := plan.Waves()
	if err != nil {
		return PlanResult{Err: err.Error()}
	}
	if exec == nil {
		return PlanResult{Err: "orchestrate: exec is required"}
	}
	out := PlanResult{Waves: len(waves)}
	done := map[string]string{} // task id -> status
	for _, wave := range waves {
		results := make([]TaskResult, len(wave))
		var wg sync.WaitGroup
		for i, task := range wave {
			if reason, skip := blocked(task, done, ctx.Err()); skip {
				results[i] = TaskResult{TaskID: task.ID, Status: StatusSkipped, Reason: reason}
				continue
			}
			wg.Add(1)
			go func(i int, task model.Task) {
				defer wg.Done()
				results[i] = r.runTask(ctx, task, exec)
			}(i, task)
		}
		wg.Wait()
		for _, res := range results {
			done[res.TaskID] = res.Status
			switch res.Status {
			case StatusSucceeded:
				out.Succeeded++
			case StatusFailed:
				out.Failed++
			default:
				out.Skipped++
			}
			out.Tasks = append(out.Tasks, res)
		}
	}
	return out
}

func blocked(task model.Task, done map[string]string, ctxErr error) (string, bool) {
	if ctxErr != nil {
		return "context: " + ctxErr.Error(), true
	}
	for _, d := range task.DependsOn {
		if st := done[d]; st != StatusSucceeded {
			return fmt.Sprintf("dependency %q %s", d, st), true
		}
	}
	return "", false
}

// runTask executes one task with one retry on error, then reflects on it.
func (r *Runner) runTask(ctx context.Context, task model.Task, exec Exec) TaskResult {
	var res TaskResult
	var err error
	var attempts []Attempt
	for n := 0; n <= MaxRetries; n++ {
		kind := AttemptExec
		if n > 0 {
			kind = AttemptRetry
		}
		res, err = exec(ctx, task)
		a := Attempt{N: len(attempts) + 1, Kind: kind}
		if err != nil {
			a.Err = err.Error()
		}
		attempts = append(attempts, a)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			break
		}
	}
	res.TaskID = task.ID
	if err != nil {
		res.Status = StatusFailed
		res.Reason = fmt.Sprintf("exec failed after %d attempts: %v", len(attempts), err)
		res.Attempts = attempts
		return res
	}
	res.Status = StatusSucceeded
	res.Reason = ""
	res.Attempts = attempts
	r.reflect(ctx, task, &res)
	return res
}

// reflect scores the result and revises it while the score is below
// PassScore, at most MaxRevisions times. A scoring or revision error ends
// reflection and is recorded on the attempt; the task stays succeeded with
// the last good output.
func (r *Runner) reflect(ctx context.Context, task model.Task, res *TaskResult) {
	if r.Score == nil {
		return
	}
	for revision := 0; ; revision++ {
		last := &res.Attempts[len(res.Attempts)-1]
		score, critique, err := r.Score(ctx, task, *res)
		if err != nil {
			last.Err = "score: " + err.Error()
			return
		}
		last.Scored = true
		last.Score = score
		last.Critique = critique
		res.Score = score
		r.auditScore(ctx, task, *last, revision, score >= PassScore)
		if score >= PassScore || r.Revise == nil || revision >= MaxRevisions {
			return
		}
		revised, err := r.Revise(ctx, task, critique)
		a := Attempt{N: len(res.Attempts) + 1, Kind: AttemptRevision}
		if err != nil {
			a.Err = err.Error()
			res.Attempts = append(res.Attempts, a)
			return
		}
		res.Attempts = append(res.Attempts, a)
		res.Output = revised.Output
	}
}

func (r *Runner) auditScore(ctx context.Context, task model.Task, a Attempt, revision int, passed bool) {
	if r.Auditor == nil {
		return
	}
	_, _ = r.Auditor.Record(ctx, model.AuditRecord{
		Time: r.now().UTC(), RunID: r.RunID, TaskID: task.ID, Agent: task.Agent, Kind: "score",
		Hashes: map[string]string{"critique": sha256Hex(a.Critique)},
		Meta: map[string]any{
			"attempt":   a.N,
			"kind":      a.Kind,
			"revision":  revision,
			"score":     a.Score,
			"threshold": PassScore,
			"passed":    passed,
			"task_kind": task.Kind,
			"wave":      task.Wave,
		},
	})
}

// Delegation bounds and reason codes. The codes are the policy engine's so
// an orchestration refusal and a policy denial read the same in the audit
// log.
const (
	// MaxDepth is the deepest delegation a task may reach.
	MaxDepth = 3
	// ReasonDepthExceeded is the policy engine's DEPTH_EXCEEDED.
	ReasonDepthExceeded = policy.ReasonDepthExceeded
	// ReasonCycleDetected is the policy engine's CYCLE_DETECTED.
	ReasonCycleDetected = policy.ReasonCycleDetected
	// ReasonDelegated is the allow reason of a successful delegation.
	ReasonDelegated = "DELEGATED"
)

// DelegationError is returned when a delegation is refused.
type DelegationError struct {
	Reason    string
	Rationale string
}

func (e *DelegationError) Error() string { return "orchestrate: " + e.Reason + ": " + e.Rationale }

// Delegate creates a child task of parent for agent with the given
// objective. Depth increments, the parent's agent joins the child's
// ancestry, and the call is refused when the new depth exceeds MaxDepth
// (DEPTH_EXCEEDED) or the agent already appears in the ancestry
// (CYCLE_DETECTED). The decision is audited as a delegation record.
func (r *Runner) Delegate(parent model.Task, agent, objective string) (model.Task, error) {
	return r.DelegateContext(context.Background(), parent, agent, objective)
}

// DelegateContext is Delegate with a context for the audit record.
func (r *Runner) DelegateContext(ctx context.Context, parent model.Task, agent, objective string) (model.Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	ancestors := append(append([]string(nil), r.lineage[parent.ID]...), parent.Agent)
	child, derr := Delegate(parent, ancestors, agent, objective)
	dec := model.Decision{Effect: model.EffectAllow, Reason: ReasonDelegated,
		Rationale: fmt.Sprintf("Agent %q delegates to %q at depth %d.", parent.Agent, agent, child.Depth)}
	if derr != nil {
		dec = model.Decision{Effect: model.EffectDeny, Reason: derr.Reason, Rationale: derr.Rationale}
	} else {
		if r.lineage == nil {
			r.lineage = map[string][]string{}
		}
		r.lineage[child.ID] = ancestors
	}
	r.auditDelegation(ctx, parent, child, agent, objective, ancestors, dec)
	if derr != nil {
		return model.Task{}, derr
	}
	return child, nil
}

// Ancestry returns the agent ids above task, nearest parent last. Only
// tasks created by Delegate have one.
func (r *Runner) Ancestry(task model.Task) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.lineage[task.ID]...)
}

// Delegate is the pure form: ancestors are the agent ids above parent,
// nearest last, and must already include parent.Agent when the caller
// tracks lineage. The child inherits Kind and Lab, has Parent set to the
// parent id, Depth parent.Depth+1, and an id derived from the parent id,
// the agent and a digest of the objective.
func Delegate(parent model.Task, ancestors []string, agent, objective string) (model.Task, *DelegationError) {
	if parent.ID == "" || agent == "" {
		return model.Task{}, &DelegationError{Reason: "INVALID_DELEGATION", Rationale: "parent id and agent are required"}
	}
	depth := parent.Depth + 1
	if depth > MaxDepth {
		return model.Task{}, &DelegationError{Reason: ReasonDepthExceeded,
			Rationale: fmt.Sprintf("Delegation depth %d exceeds the maximum of %d.", depth, MaxDepth)}
	}
	chain := append([]string(nil), ancestors...)
	if parent.Agent != "" && !containsStr(chain, parent.Agent) {
		chain = append(chain, parent.Agent)
	}
	if containsStr(chain, agent) {
		return model.Task{}, &DelegationError{Reason: ReasonCycleDetected,
			Rationale: fmt.Sprintf("Agent %q already appears in its own delegation chain.", agent)}
	}
	return model.Task{
		ID:        fmt.Sprintf("%s/%s-%s", parent.ID, agent, sha256Hex(objective)[:8]),
		Wave:      parent.Wave,
		DependsOn: nil,
		Kind:      parent.Kind,
		Agent:     agent,
		Depth:     depth,
		Parent:    parent.ID,
		Lab:       parent.Lab,
	}, nil
}

func (r *Runner) auditDelegation(ctx context.Context, parent, child model.Task, agent, objective string, ancestors []string, dec model.Decision) {
	if r.Auditor == nil {
		return
	}
	_, _ = r.Auditor.Record(ctx, model.AuditRecord{
		Time: r.now().UTC(), RunID: r.RunID, TaskID: parent.ID, Agent: parent.Agent, Kind: "delegation",
		Hashes: map[string]string{"objective": sha256Hex(objective)},
		Meta: map[string]any{
			"effect":    string(dec.Effect),
			"reason":    dec.Reason,
			"rationale": dec.Rationale,
			"to_agent":  agent,
			"child_id":  child.ID,
			"depth":     parent.Depth + 1,
			"ancestors": len(ancestors),
			"max_depth": MaxDepth,
		},
	})
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// IsDelegationRefused reports whether err is a refused delegation and, if
// so, returns its reason code.
func IsDelegationRefused(err error) (string, bool) {
	var de *DelegationError
	if errors.As(err, &de) {
		return de.Reason, true
	}
	return "", false
}
