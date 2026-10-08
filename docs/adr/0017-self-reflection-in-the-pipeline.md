# ADR 0017: Self-reflection runs inside the pipeline and gates revision only

Status: accepted (2026-10-08)

## Context

ADR 0015 added wave orchestration with a self-reflection step as library code.
`assay run` did not use it, so a claim such as "each model scores its own work
and revises when the score is low" described the library, not the pipeline.

## Decision

`assay run` treats the probe of one lab by one model as an orchestrated task.

- The model assesses the lab as before. The same model is then asked, in a
  separate conversation under the `score` agent, to rate that report from 0 to
  10 through the signed `score_task` tool. It is shown only the objective and
  a list of class, method and path template per finding, never response bodies.
- A score below 7 sends the model back with its own critique, at most twice.
  Revision findings are merged with the earlier ones by dedup key.
- Every agent run, including each revision and the scoring turn, gets its own
  task id, so tool call ids that a model reuses across conversations never
  collide in reconciliation (ADR 0014).
- Reflection is skipped, and recorded as skipped, when the model declined or
  the budget was reached. A model that reports no score is recorded as such;
  no score is invented.
- The run report carries one `ReflectionResult` per lab and model: scores in
  order, revision count, final score, and a note when reflection did not
  happen. Critique text is not stored anywhere; the audit log holds its hash.
- A score never changes a finding's state. Only a deterministic checker can
  mark a finding validated or refuted (ADR 0010).

## Consequences

A model scoring its own report is a biased judge, and nothing here claims
otherwise. The record exists so the bias can be measured: self-score against
the validated count and, on labs with complete ground truth, against recall.
Whether a model's score tracks its results is a finding to publish when there
are runs with several real models, not an assumption.

Scoring and revision add model calls, so a run costs more and takes longer,
bounded by the router's budgets.
