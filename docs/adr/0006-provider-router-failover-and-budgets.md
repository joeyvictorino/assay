# ADR 0006: Provider router, failover and budgets

## Status

Accepted (2026-10-07).

## Context

assay runs several models over the same targets and attributes every
finding to the model that produced it. Model APIs fail transiently (429,
5xx, timeouts), prices differ by an order of magnitude between models, and
a runaway agent loop can spend without bound. The harness needs one place
that chooses a model per task, keeps spend inside published caps, and
records why each choice was made.

## Decision

- `internal/provider` defines `model.Provider` backends (Anthropic SDK,
  hand-rolled OpenAI-compatible client for OpenAI and llama-server, and a
  scripted fake), a cost table loaded from `runs/*.yaml`
  (`cost_per_mtok`), `ComputeCost`, and `Classify(err)`: 408/409/425/429,
  5xx, timeouts and connection failures are retryable; 400/401/403/404/422
  are not.
- Providers do not retry internally (SDK max retries is 0). The router is
  the only component that fails over, so per-model attribution stays exact
  and server-side fallbacks are never used.
- `internal/router` holds ordered candidates per task kind. It picks the
  first candidate whose per-model budget has room, fails over to the next
  candidate at most once per call on a retryable error (configurable via
  `max_failovers`), and enforces per-task latency through a context
  deadline with a per-model override for slow local models.
- Three USD caps are enforced before every call: per run, per model, per
  task. Spend is tracked under a mutex and `ErrBudgetExceeded` is returned
  when no candidate has room. Defaults: 15 / 6 / 1.50 USD.
- Every call is audited as `model_call` with digests only (prompt hash,
  response hash, claimed tool-call ids and argument digests) plus tokens,
  cache reads, latency and micro-USD cost. Every choice is audited as
  `router_decision` with candidates, chosen, reason and remaining budget.
- A `Declined` response is returned immediately: no failover, no retry (see
  ADR 0009).

## Consequences

- A model outage degrades to the next candidate rather than failing the
  run, but at most one hop per call keeps latency and cost predictable.
- Prices live in configuration, so a price change is a config commit and
  every published cost figure is reproducible from `runs/*.yaml`.
- Budget enforcement is pre-call, so a single call may overshoot a cap by
  at most one call's cost; caps are set with that margin in mind.
- Operators read failover and budget behaviour from the audit log without
  access to any prompt or response text.

## Amendment, 2026-10-10: caps across routers

`assay run` builds one router per (lab, model) assessment, and a router
counts only the spend it made, so the run and model caps would have applied
to each assessment separately. The run command now gives every new router
what is left of the run and model caps (the per-task cap is unchanged) and
does not start an assessment when either is used up; that makes the run
`BLOCKED` (exit 1, `truncated: true`) instead of letting it pass. The
one-call overshoot above still applies. `TestRunAndModelBudgetsHoldAcrossLabs`
pins this.
