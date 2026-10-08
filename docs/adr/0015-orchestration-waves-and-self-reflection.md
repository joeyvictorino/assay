# ADR 0015: Orchestration with waves, dependencies and self-reflection

Status: accepted (2026-10-07)

## Context

A validation run is several tasks with ordering constraints: reconnaissance
before probing, probing before validation, validation before remediation.
Model output is uneven, so a task that finishes is not necessarily a task
that finished well. Delegation between agents must not be able to recurse or
loop.

## Decision

`internal/orchestrate` runs a plan wave by wave.

- A plan is a list of `model.Task` with `DependsOn`. `Validate` rejects an
  unknown dependency, a self-dependency and a cycle, and names the cycle
  path. `Waves` returns topological levels in a deterministic order.
- A task whose dependency failed or was skipped is marked `skipped` with the
  reason; it never runs on partial inputs.
- After each task a `Score` callback returns 0 to 10 with a critique. A score
  below 7 triggers one `Revise` call, at most twice. Every attempt, with its
  score, is kept in `TaskResult.Attempts`. Critique text is hashed in the
  audit log, not stored.
- An execution error is retried once before the task is marked failed.
- Delegation increments `Depth` and extends the ancestry. Depth above 3 is
  refused with `DEPTH_EXCEEDED` and an agent already in the ancestry with
  `CYCLE_DETECTED`, the same codes the policy engine uses (ADR 0003).
- Delegation and score events are audit records of kind `delegation` and
  `score` (ADR 0004).

## Consequences

The orchestrator is pure control flow over callbacks, so it is tested with
scripted executors and needs no model. The 7 threshold and the limits of two
revisions and depth 3 are constants of this ADR; changing them is a new ADR.
A score is a model's opinion of its own work, so it gates revision, never
validation: only a deterministic checker can mark a finding validated
(ADR 0010).
