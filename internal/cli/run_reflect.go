package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/joeyvictorino/assay/internal/agent"
	"github.com/joeyvictorino/assay/internal/config"
	"github.com/joeyvictorino/assay/internal/finding"
	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/labs"
	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/orchestrate"
	"github.com/joeyvictorino/assay/internal/redact"
	"github.com/joeyvictorino/assay/internal/router"
	"github.com/joeyvictorino/assay/internal/tools"
	"github.com/joeyvictorino/assay/internal/zdr"
)

// The probe of one lab by one model is an orchestrated task: the model
// assesses the lab, then scores its own report from 0 to 10 through the
// signed score_task tool. A score below orchestrate.PassScore sends the
// model back with its own critique, at most orchestrate.MaxRevisions times.
// The score gates revision only. A finding becomes validated only when a
// deterministic checker says so (ADR 0010, ADR 0015).

const scoreSystemPrompt = `You are reviewing a security assessment of an intentionally vulnerable training application. Judge only the list of findings you are given against the stated objective: coverage of the visible attack surface and how specific the findings are. Rate it from 0 to 10 and call score_task exactly once with an integer score and a one-sentence critique naming what is missing. Do not run any other tool.`

// probeInputs is everything one (lab, model) assessment needs.
type probeInputs struct {
	cfg        *config.Config
	rt         *router.Router
	lab        labs.Lab
	labName    string
	ref        model.ModelRef
	client     *httpx.Client
	gate       model.Gate
	ts         model.ManifestVerifier
	eng        model.PolicyEvaluator
	aw         model.Auditor
	probeTools []model.ToolManifest
	scoreTools []model.ToolManifest
	runID      string
	stderr     io.Writer
}

type probeOutcome struct {
	Findings   []model.Finding
	Usage      model.Usage
	Refusals   int
	Failovers  int
	Truncated  bool
	Reflection model.ReflectionResult
	Err        error
}

type scoreCall struct {
	score    int
	critique string
}

func runProbeTask(ctx context.Context, in probeInputs) probeOutcome {
	out := probeOutcome{Reflection: model.ReflectionResult{Lab: in.labName, Model: in.ref.Model}}
	acc := map[string]model.Finding{}
	var order []string
	merge := func(fs []model.Finding) {
		for _, f := range fs {
			if have, ok := acc[f.DedupKey]; ok {
				acc[f.DedupKey] = finding.Merge(have, f)
				continue
			}
			acc[f.DedupKey] = f
			order = append(order, f.DedupKey)
		}
	}
	base := in.labName + ":" + in.ref.Model
	// Every agent run gets its own task id: tool call ids are unique within
	// one conversation, and a revision or retry is a new conversation
	// (ADR 0014).
	runs := map[string]int{}
	taskID := func(kind string) string {
		runs[kind]++
		id := base
		if kind != "probe" {
			id += ":" + kind
		}
		if runs[kind] > 1 {
			id += fmt.Sprintf("#%d", runs[kind])
		}
		return id
	}
	account := func(res agent.Result) {
		out.Usage = addUsage(out.Usage, res.Usage)
		out.Failovers += res.Failovers
		if res.Declined {
			out.Refusals++
		}
	}
	runAgent := func(ctx context.Context, objective string) error {
		var reported []model.Finding
		res, err := agent.Run(ctx, agent.Params{
			Router: in.rt, TaskKind: "probe", TaskID: taskID("probe"), Agent: "probe",
			Lab: in.labName, RunID: in.runID, Objective: objective, System: systemPrompt,
			Tools: in.probeTools, Verifier: in.ts, Policy: in.eng, Auditor: in.aw,
			Executor: tools.NewExecutor(),
			Env: tools.Env{HTTP: in.client.Do, Gate: in.gate, Lab: in.labName, RunID: in.runID, Model: in.ref,
				Report: func(f model.Finding) { reported = append(reported, f) }, Sanitize: redact.Sanitize},
			MaxTurns: in.cfg.Agent.MaxTurns, MaxTokens: in.cfg.Agent.MaxTokens, Sink: zdr.NullSink{},
		})
		account(res)
		fs := res.Findings
		if len(fs) == 0 {
			fs = reported
		}
		merge(fs)
		if errors.Is(err, router.ErrBudgetExceeded) {
			out.Truncated = true
		}
		return err
	}
	summary := func() string {
		keys := append([]string(nil), order...)
		sort.Strings(keys)
		var b strings.Builder
		fmt.Fprintf(&b, "%d findings reported", len(keys))
		for i, k := range keys {
			if i >= 40 {
				fmt.Fprintf(&b, "\n... and %d more", len(keys)-i)
				break
			}
			f := acc[k]
			fmt.Fprintf(&b, "\n- %s %s %s", f.Class, f.Location.Method, f.Location.PathTemplate)
		}
		return b.String()
	}

	exec := func(ctx context.Context, _ model.Task) (orchestrate.TaskResult, error) {
		err := runAgent(ctx, objective(in.lab))
		if err != nil {
			fmt.Fprintf(in.stderr, "run: %s on %s: %v\n", in.ref.Model, in.labName, err)
		}
		return orchestrate.TaskResult{Output: summary()}, err
	}
	score := func(ctx context.Context, _ model.Task, res orchestrate.TaskResult) (int, string, error) {
		if out.Refusals > 0 || out.Truncated {
			return 0, "", errors.New("reflection skipped: the model declined or the budget was reached")
		}
		var got *scoreCall
		sres, err := agent.Run(ctx, agent.Params{
			Router: in.rt, TaskKind: "score", TaskID: taskID("score"), Agent: "score",
			Lab: in.labName, RunID: in.runID,
			Objective: fmt.Sprintf("Objective: %s\n\n%s", objective(in.lab), res.Output),
			System:    scoreSystemPrompt,
			Tools:     in.scoreTools, Verifier: in.ts, Policy: in.eng, Auditor: in.aw,
			Executor: tools.NewExecutor(),
			Env: tools.Env{Gate: in.gate, Lab: in.labName, RunID: in.runID, Model: in.ref,
				Score:    func(s int, c string) { got = &scoreCall{score: s, critique: c} },
				Sanitize: redact.Sanitize},
			MaxTurns: 3, MaxTokens: in.cfg.Agent.MaxTokens, Sink: zdr.NullSink{},
		})
		account(sres)
		if err != nil {
			return 0, "", err
		}
		if got == nil {
			return 0, "", errors.New("the model did not report a score")
		}
		return got.score, got.critique, nil
	}
	revise := func(ctx context.Context, _ model.Task, critique string) (orchestrate.TaskResult, error) {
		err := runAgent(ctx, fmt.Sprintf("%s\n\nA reviewer rated your previous report below the pass mark: %s\nTest what was missed, and report only findings you have not already reported.", objective(in.lab), critique))
		return orchestrate.TaskResult{Output: summary()}, err
	}

	runner := &orchestrate.Runner{Score: score, Revise: revise, Auditor: in.aw, RunID: in.runID}
	task := model.Task{ID: base, Kind: "probe", Agent: "probe", Lab: in.labName}
	pr := runner.Run(ctx, orchestrate.Plan{Tasks: []model.Task{task}}, exec)
	if pr.Err != "" {
		out.Err = errors.New(pr.Err)
	}
	for _, tr := range pr.Tasks {
		if tr.Status == orchestrate.StatusFailed {
			out.Err = errors.New(tr.Reason)
		}
		for _, a := range tr.Attempts {
			if a.Kind == orchestrate.AttemptRevision {
				out.Reflection.Revisions++
			}
			if a.Scored {
				out.Reflection.Scored = true
				out.Reflection.Scores = append(out.Reflection.Scores, a.Score)
			}
			// The orchestrator records a scoring error on whichever attempt
			// came last, so the prefix is checked before the attempt kind.
			switch {
			case strings.HasPrefix(a.Err, "score: "):
				out.Reflection.Note = noteFor("score failed", strings.TrimPrefix(a.Err, "score: "))
			case a.Err != "" && a.Kind == orchestrate.AttemptRevision:
				out.Reflection.Note = noteFor("revision failed", a.Err)
			}
		}
		if out.Reflection.Scored {
			out.Reflection.Final = tr.Score
			out.Reflection.Passed = tr.Score >= orchestrate.PassScore
		}
	}
	for _, k := range order {
		out.Findings = append(out.Findings, acc[k])
	}
	return out
}

// noteFor builds a short, redacted explanation for the run report.
func noteFor(what, err string) string {
	err = redact.Sanitize(strings.Join(strings.Fields(err), " "))
	if len(err) > 160 {
		err = err[:160] + "..."
	}
	return what + ": " + err
}
