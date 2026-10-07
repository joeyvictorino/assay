# ADR 0009: Refusals are recorded, never retried

## Status

Accepted (2026-10-07).

## Context

Current models return `stop_reason: refusal` (Anthropic) or
`finish_reason: content_filter` (OpenAI-compatible APIs) when a safety
classifier declines a request. A security-validation harness sits close to
the boundary those classifiers police. Rephrasing a declined prompt,
retrying it, or routing it to a model that does not decline would turn the
harness into a tool for working around safety systems, and would also
corrupt the per-model comparison the project exists to publish.

## Decision

- Providers check the refusal stop reason before reading any content and
  return `model.Response{Declined: true, StopReason: "refusal"}` with a nil
  error. Refusal text and partial content are discarded.
- The router returns a declined response immediately. It is not a transport
  error, so `Classify` never marks it retryable; no failover to another
  candidate occurs. The call is still audited (`model_call` with
  `declined: true`) and still charged against budgets.
- The agent loop stops on the first declined turn with `Result.Declined =
  true` and no further requests. It does not rephrase the objective, drop
  tools, or re-ask.
- Server-side fallback parameters are never sent, because a fallback would
  silently change which model answered.
- Refusal counts are reported per model in the run report (`Refusals`), so
  a model that declines a lab is visible as data rather than hidden by a
  retry.

## Consequences

- Some tasks finish with no finding content for some models. That is a
  measured outcome, not a failure to be engineered away.
- Operators can audit that no declined request was re-sent: the audit log
  shows exactly one `model_call` per declined turn and no subsequent call
  for that task.
- Tests `TestRouterNeverRetriesDeclined` and `TestRefusalStopsImmediately`
  guard this behaviour.
