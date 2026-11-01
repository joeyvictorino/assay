# ADR 0018: A peer may score a model's report instead of the model itself

Status: accepted (2026-11-01)

## Context

ADR 0017 has each model score its own report and says plainly that a model
grading itself is a biased judge. Multi-agent coding harnesses that hold up
under scrutiny separate the roles: the worker that produced a result does not
decide whether it is good; a manager with a fresh context reads the captured
output and decides. The same separation is cheap here, because a run already
has several models and every scoring turn is its own conversation with no
shared context.

## Decision

A run config may name who scores a report: `reflection: { reviewer: self }`
(the default, ADR 0017 unchanged) or `reflection: { reviewer: peer }`.

- With `peer`, the reviewer for a (lab, model) task is the first model in the
  run's `models` list other than the assessed model, in list order. List
  order keeps the choice a pure function of the config file, so a run is
  reproducible from the file alone. A run with a single model has no peer:
  it falls back to self and the reflection record says so.
- The reviewer runs under the `score` agent in a separate conversation and
  sees only the objective and the list of class, method and path template per
  finding, never response bodies, exactly as in ADR 0017. Its system prompt
  says it is judging another model's work and must not defend it.
- `model.ReflectionResult` names the reviewer (`reviewer`, a model id) and its
  kind (`reviewer_kind`: `self` or `peer`). The run report's self-reflection
  table shows both.
- Spend: a task's router charges every call of that task, including a peer
  reviewer's scoring turns, against the assessed model's per-model cap and the
  run cap, as it already does for self-scoring. The caps therefore bind
  earlier with a peer reviewer, never later. Token usage is accounted the same
  way. Per-reviewer spend is recoverable from the audit log's `model_call`
  records, which name the model.
- The scripted `fake` provider is built per provider name, so a run with two
  fake providers gives the assessed model and its reviewer separate scripts.

Nothing changes about what a score does. It gates revision only (ADR 0015);
a finding becomes validated or refuted through a deterministic checker and
nothing else (ADR 0010).

## Consequences

A peer is a less biased judge than the author, and still a model's opinion.
The record exists so the two can be compared: once runs with several real
models exist, self-scores and peer scores for the same report can be set
beside the validated count and, on labs with complete ground truth, beside
recall. Which reviewer tracks the checkers better is a result to publish, not
an assumption.

Peer review adds nothing to a run's cost beyond what self-scoring already
costs; it moves the scoring calls to another model. A reviewer that declines
or reports no score is recorded, never invented (ADR 0009, ADR 0017).

The reflection record now also carries what the checkers said about the same
model's findings on that lab: `validated`, `refuted` and `theorized` counts
after validation and reconciliation. This is the amendment to ADR 0017 made
at the same time: a score sits next to the evidence it was supposed to
anticipate, so a passed self-score over refuted findings is visible in every
report rather than only to a script.
