package orchestrate

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/policy"
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

func (a *memAuditor) byKind(kind string) []model.AuditRecord {
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

func okExec(_ context.Context, t model.Task) (TaskResult, error) {
	return TaskResult{Output: "out-" + t.ID}, nil
}

func statuses(pr PlanResult) map[string]string {
	out := map[string]string{}
	for _, r := range pr.Tasks {
		out[r.TaskID] = r.Status
	}
	return out
}

func TestRunDependenciesAndSkips(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name       string
		plan       Plan
		failAlways map[string]bool // tasks whose exec always errors
		failOnce   map[string]bool // tasks whose exec errors on the first call only
		want       map[string]string
		wantOrder  []string
		wantReason map[string]string // substring
	}{
		{
			name:      "all succeed in wave order",
			plan:      Plan{Tasks: []model.Task{task("c", "b"), task("b", "a"), task("a"), task("d", "a")}},
			want:      map[string]string{"a": StatusSucceeded, "b": StatusSucceeded, "c": StatusSucceeded, "d": StatusSucceeded},
			wantOrder: []string{"a", "b", "d", "c"},
		},
		{
			name:       "failure skips dependents transitively",
			plan:       Plan{Tasks: []model.Task{task("a"), task("b", "a"), task("c", "b"), task("x")}},
			failAlways: map[string]bool{"a": true},
			want:       map[string]string{"a": StatusFailed, "b": StatusSkipped, "c": StatusSkipped, "x": StatusSucceeded},
			wantReason: map[string]string{"a": "exec failed after 2 attempts", "b": `dependency "a" failed`, "c": `dependency "b" skipped`},
		},
		{
			name:     "transient error is retried once",
			plan:     Plan{Tasks: []model.Task{task("a"), task("b", "a")}},
			failOnce: map[string]bool{"a": true},
			want:     map[string]string{"a": StatusSucceeded, "b": StatusSucceeded},
		},
		{
			name:       "diamond with one failed side",
			plan:       Plan{Tasks: []model.Task{task("a"), task("b", "a"), task("c", "a"), task("d", "b", "c")}},
			failAlways: map[string]bool{"c": true},
			want:       map[string]string{"a": StatusSucceeded, "b": StatusSucceeded, "c": StatusFailed, "d": StatusSkipped},
			wantReason: map[string]string{"d": `dependency "c" failed`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			calls := map[string]int{}
			exec := func(_ context.Context, task model.Task) (TaskResult, error) {
				mu.Lock()
				calls[task.ID]++
				n := calls[task.ID]
				mu.Unlock()
				if tc.failAlways[task.ID] || (tc.failOnce[task.ID] && n == 1) {
					return TaskResult{}, boom
				}
				return TaskResult{Output: "ok"}, nil
			}
			r := &Runner{}
			pr := r.Run(context.Background(), tc.plan, exec)
			if pr.Err != "" {
				t.Fatalf("Err = %q", pr.Err)
			}
			got := statuses(pr)
			for id, want := range tc.want {
				if got[id] != want {
					t.Errorf("task %s status = %q, want %q (reason %q)", id, got[id], want, reasonOf(pr, id))
				}
			}
			for id, sub := range tc.wantReason {
				if !strings.Contains(reasonOf(pr, id), sub) {
					t.Errorf("task %s reason = %q, want substring %q", id, reasonOf(pr, id), sub)
				}
			}
			if tc.wantOrder != nil {
				var order []string
				for _, res := range pr.Tasks {
					order = append(order, res.TaskID)
				}
				if strings.Join(order, ",") != strings.Join(tc.wantOrder, ",") {
					t.Errorf("order = %v, want %v", order, tc.wantOrder)
				}
			}
			for id := range tc.failOnce {
				if calls[id] != 2 {
					t.Errorf("task %s exec calls = %d, want 2", id, calls[id])
				}
				if n := len(attemptsOf(pr, id)); n != 2 {
					t.Errorf("task %s attempts = %d, want 2", id, n)
				}
			}
			for id := range tc.failAlways {
				if calls[id] != 1+MaxRetries {
					t.Errorf("task %s exec calls = %d, want %d", id, calls[id], 1+MaxRetries)
				}
			}
			if pr.Succeeded+pr.Failed+pr.Skipped != len(tc.plan.Tasks) {
				t.Errorf("counts %d/%d/%d do not sum to %d", pr.Succeeded, pr.Failed, pr.Skipped, len(tc.plan.Tasks))
			}
		})
	}
}

func reasonOf(pr PlanResult, id string) string {
	for _, r := range pr.Tasks {
		if r.TaskID == id {
			return r.Reason
		}
	}
	return ""
}

func attemptsOf(pr PlanResult, id string) []Attempt {
	for _, r := range pr.Tasks {
		if r.TaskID == id {
			return r.Attempts
		}
	}
	return nil
}

func TestRunInvalidPlanAndNilExec(t *testing.T) {
	r := &Runner{}
	pr := r.Run(context.Background(), Plan{Tasks: []model.Task{task("a", "a")}}, okExec)
	if pr.Err == "" || len(pr.Tasks) != 0 {
		t.Fatalf("invalid plan result = %+v", pr)
	}
	pr = r.Run(context.Background(), Plan{Tasks: []model.Task{task("a")}}, nil)
	if !strings.Contains(pr.Err, "exec is required") {
		t.Fatalf("nil exec result = %+v", pr)
	}
}

func TestRunCancelledContextSkips(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	plan := Plan{Tasks: []model.Task{task("a"), task("b", "a")}}
	exec := func(ctx context.Context, task model.Task) (TaskResult, error) {
		cancel()
		return TaskResult{Output: "ok"}, nil
	}
	pr := (&Runner{}).Run(ctx, plan, exec)
	got := statuses(pr)
	if got["a"] != StatusSucceeded || got["b"] != StatusSkipped || !strings.Contains(reasonOf(pr, "b"), "context") {
		t.Fatalf("result = %+v", pr)
	}
}

func TestReflection(t *testing.T) {
	cases := []struct {
		name          string
		scores        []int // returned by successive Score calls
		scoreErrAt    int   // 1-based call index that errors; 0 none
		reviseErr     bool  // revise returns an error
		noRevise      bool  // Runner.Revise nil
		wantAttempts  int   // total attempts recorded
		wantKinds     []string
		wantFinal     int    // TaskResult.Score
		wantRevisions int    // revise calls
		wantOutput    string // final output
		wantScored    []bool // per attempt
		wantAuditN    int    // score records
	}{
		{"passes first time", []int{9}, 0, false, false, 1, []string{AttemptExec}, 9, 0, "v0", []bool{true}, 1},
		{"one revision then passes", []int{5, 8}, 0, false, false, 2, []string{AttemptExec, AttemptRevision}, 8, 1, "v1", []bool{true, true}, 2},
		{"two revisions cap", []int{3, 4, 5}, 0, false, false, 3, []string{AttemptExec, AttemptRevision, AttemptRevision}, 5, 2, "v2", []bool{true, true, true}, 3},
		{"exactly seven passes", []int{7}, 0, false, false, 1, []string{AttemptExec}, 7, 0, "v0", []bool{true}, 1},
		{"low score without revise", []int{2}, 0, false, true, 1, []string{AttemptExec}, 2, 0, "v0", []bool{true}, 1},
		{"score error ends reflection", []int{0}, 1, false, false, 1, []string{AttemptExec}, 0, 0, "v0", []bool{false}, 0},
		{"revise error recorded", []int{4}, 0, true, false, 2, []string{AttemptExec, AttemptRevision}, 4, 1, "v0", []bool{true, false}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var scoreCalls, reviseCalls int
			aud := &memAuditor{}
			r := &Runner{
				Auditor: aud,
				RunID:   "r1",
				Now:     func() time.Time { return time.Unix(0, 0) },
				Score: func(_ context.Context, _ model.Task, res TaskResult) (int, string, error) {
					scoreCalls++
					if tc.scoreErrAt == scoreCalls {
						return 0, "", errors.New("judge down")
					}
					return tc.scores[scoreCalls-1], "critique-" + res.Output, nil
				},
			}
			if !tc.noRevise {
				r.Revise = func(_ context.Context, task model.Task, critique string) (TaskResult, error) {
					reviseCalls++
					if tc.reviseErr {
						return TaskResult{}, errors.New("cannot revise")
					}
					if !strings.HasPrefix(critique, "critique-") {
						t.Errorf("critique = %q", critique)
					}
					return TaskResult{Output: "v" + string(rune('0'+reviseCalls))}, nil
				}
			}
			exec := func(_ context.Context, _ model.Task) (TaskResult, error) { return TaskResult{Output: "v0"}, nil }
			pr := r.Run(context.Background(), Plan{Tasks: []model.Task{task("a")}}, exec)
			res := pr.Tasks[0]
			if res.Status != StatusSucceeded {
				t.Fatalf("status = %s reason %s", res.Status, res.Reason)
			}
			if len(res.Attempts) != tc.wantAttempts {
				t.Fatalf("attempts = %+v, want %d", res.Attempts, tc.wantAttempts)
			}
			for i, a := range res.Attempts {
				if a.Kind != tc.wantKinds[i] || a.N != i+1 || a.Scored != tc.wantScored[i] {
					t.Errorf("attempt %d = %+v, want kind %s scored %v", i, a, tc.wantKinds[i], tc.wantScored[i])
				}
				if a.Scored && !strings.HasPrefix(a.Critique, "critique-") {
					t.Errorf("attempt %d critique = %q", i, a.Critique)
				}
			}
			if res.Score != tc.wantFinal || reviseCalls != tc.wantRevisions || res.Output != tc.wantOutput {
				t.Errorf("score=%d revisions=%d output=%q, want %d/%d/%q", res.Score, reviseCalls, res.Output, tc.wantFinal, tc.wantRevisions, tc.wantOutput)
			}
			recs := aud.byKind("score")
			if len(recs) != tc.wantAuditN {
				t.Fatalf("score records = %d, want %d", len(recs), tc.wantAuditN)
			}
			for _, rec := range recs {
				if rec.TaskID != "a" || rec.RunID != "r1" || rec.Agent != "probe" {
					t.Errorf("record = %+v", rec)
				}
				if h := rec.Hashes["critique"]; len(h) != 64 || strings.Contains(h, "critique") {
					t.Errorf("critique hash = %q", h)
				}
				for k, v := range rec.Meta {
					if s, ok := v.(string); ok && strings.Contains(s, "critique-") {
						t.Errorf("meta %s leaks critique text: %q", k, s)
					}
				}
				if _, ok := rec.Meta["score"].(int); !ok {
					t.Errorf("meta score = %#v", rec.Meta["score"])
				}
			}
		})
	}
}

func TestDelegate(t *testing.T) {
	root := model.Task{ID: "root", Kind: "probe", Agent: "lead", Lab: "synthetic-ops", Depth: 0}
	cases := []struct {
		name       string
		parent     model.Task
		ancestors  []string
		agent      string
		wantReason string // "" means allowed
		wantDepth  int
	}{
		{"first hop", root, nil, "recon", "", 1},
		{"depth three allowed", model.Task{ID: "p", Agent: "c", Depth: 2}, []string{"a", "b"}, "d", "", 3},
		{"depth four refused", model.Task{ID: "p", Agent: "c", Depth: 3}, []string{"a", "b"}, "e", ReasonDepthExceeded, 0},
		{"cycle via parent agent", root, nil, "lead", ReasonCycleDetected, 0},
		{"cycle via ancestor", model.Task{ID: "p", Agent: "b", Depth: 1}, []string{"a"}, "a", ReasonCycleDetected, 0},
		{"missing agent", root, nil, "", "INVALID_DELEGATION", 0},
		{"missing parent id", model.Task{Agent: "x"}, nil, "y", "INVALID_DELEGATION", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			child, derr := Delegate(tc.parent, tc.ancestors, tc.agent, "look at /login")
			if tc.wantReason != "" {
				if derr == nil || derr.Reason != tc.wantReason {
					t.Fatalf("Delegate err = %v, want reason %s", derr, tc.wantReason)
				}
				if reason, ok := IsDelegationRefused(derr); !ok || reason != tc.wantReason {
					t.Fatalf("IsDelegationRefused = %q,%v", reason, ok)
				}
				return
			}
			if derr != nil {
				t.Fatal(derr)
			}
			if child.Depth != tc.wantDepth || child.Parent != tc.parent.ID || child.Agent != tc.agent {
				t.Fatalf("child = %+v", child)
			}
			if child.Kind != tc.parent.Kind || child.Lab != tc.parent.Lab {
				t.Fatalf("child did not inherit kind/lab: %+v", child)
			}
			if !strings.HasPrefix(child.ID, tc.parent.ID+"/"+tc.agent+"-") || strings.Contains(child.ID, "login") {
				t.Fatalf("child id = %q", child.ID)
			}
		})
	}
}

func TestDelegateMirrorsPolicyCodes(t *testing.T) {
	if ReasonDepthExceeded != policy.ReasonDepthExceeded || ReasonCycleDetected != policy.ReasonCycleDetected {
		t.Fatalf("codes drifted: %s %s", ReasonDepthExceeded, ReasonCycleDetected)
	}
	if ReasonDepthExceeded != "DEPTH_EXCEEDED" || ReasonCycleDetected != "CYCLE_DETECTED" {
		t.Fatalf("codes = %s %s", ReasonDepthExceeded, ReasonCycleDetected)
	}
}

func TestRunnerDelegateTracksLineageAndAudits(t *testing.T) {
	aud := &memAuditor{}
	r := &Runner{Auditor: aud, RunID: "r1"}
	root := model.Task{ID: "root", Kind: "probe", Agent: "lead", Lab: "l"}

	c1, err := r.Delegate(root, "recon", "map the app")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := r.Delegate(c1, "probe", "probe /login")
	if err != nil {
		t.Fatal(err)
	}
	c3, err := r.Delegate(c2, "validate", "confirm sqli")
	if err != nil {
		t.Fatal(err)
	}
	if c3.Depth != 3 {
		t.Fatalf("depth = %d", c3.Depth)
	}
	if got := strings.Join(r.Ancestry(c3), ","); got != "lead,recon,probe" {
		t.Fatalf("ancestry = %q", got)
	}
	// Depth 4 is refused.
	if _, err := r.Delegate(c3, "remediate", "fix"); err == nil {
		t.Fatal("depth 4 delegation succeeded")
	} else if reason, _ := IsDelegationRefused(err); reason != ReasonDepthExceeded {
		t.Fatalf("reason = %s", reason)
	}
	// An ancestor agent two hops up is a cycle.
	if _, err := r.Delegate(c2, "lead", "report back"); err == nil {
		t.Fatal("cycle delegation succeeded")
	} else if reason, _ := IsDelegationRefused(err); reason != ReasonCycleDetected {
		t.Fatalf("reason = %s", reason)
	}
	// A task the runner never saw has only its own agent as ancestry.
	if _, err := r.Delegate(model.Task{ID: "other", Agent: "z"}, "recon", "x"); err != nil {
		t.Fatalf("unrelated delegation refused: %v", err)
	}

	recs := aud.byKind("delegation")
	if len(recs) != 6 {
		t.Fatalf("delegation records = %d, want 6", len(recs))
	}
	var denies []string
	for _, rec := range recs {
		if rec.RunID != "r1" || rec.Kind != "delegation" {
			t.Errorf("record = %+v", rec)
		}
		if h := rec.Hashes["objective"]; len(h) != 64 {
			t.Errorf("objective hash = %q", h)
		}
		for _, v := range rec.Meta {
			if s, ok := v.(string); ok && (strings.Contains(s, "map the app") || strings.Contains(s, "probe /login")) {
				t.Errorf("meta leaks objective text: %q", s)
			}
		}
		if rec.Meta["effect"] == string(model.EffectDeny) {
			denies = append(denies, rec.Meta["reason"].(string))
		}
	}
	if strings.Join(denies, ",") != ReasonDepthExceeded+","+ReasonCycleDetected {
		t.Fatalf("denies = %v", denies)
	}
}

func TestRunConcurrentWaveIsRaceFree(t *testing.T) {
	plan := Plan{Tasks: []model.Task{task("root")}}
	for _, id := range []string{"a", "b", "c", "d", "e", "f"} {
		plan.Tasks = append(plan.Tasks, task(id, "root"))
	}
	aud := &memAuditor{}
	r := &Runner{Auditor: aud, Score: func(context.Context, model.Task, TaskResult) (int, string, error) { return 9, "fine", nil }}
	pr := r.Run(context.Background(), plan, okExec)
	if pr.Succeeded != 7 || pr.Waves != 2 {
		t.Fatalf("result = %+v", pr)
	}
	if n := len(aud.byKind("score")); n != 7 {
		t.Fatalf("score records = %d", n)
	}
	for i, res := range pr.Tasks[1:] {
		if want := string(rune('a' + i)); res.TaskID != want {
			t.Fatalf("task %d = %s, want %s", i+1, res.TaskID, want)
		}
	}
}
