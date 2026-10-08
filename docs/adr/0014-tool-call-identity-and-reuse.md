# ADR 0014: Tool call identity is per task; reuse inside a task is reported

Status: accepted (2026-10-07)

## Context

Reconciliation joins what a model claimed (the `claimed_tool_calls` list in
each `model_call` record) against what the harness executed (the `tool_call`
records). The first committed run reported duplicate tool call ids on both
sides. The cause was not a model misbehaving. The scripted provider used the
same ids in every lab, and the reconciler joined over the whole run, but a
provider guarantees id uniqueness only within one conversation. The false
mismatches also downgraded those models' findings from validated to
theorized.

## Decision

- The join key is the task id plus the tool call id. Both record kinds carry
  the task id (`<lab>:<model>` in the pipeline). The same id in two tasks is
  two calls, not a reuse.
- Inside one task the agent loop gives every executed call an id that is
  unique in an `IDSpace`. The first use of a claimed id keeps it; a later
  use is recorded as `<claimed>#<turn>` with the model's claim in
  `Meta["claimed_tool_call_id"]`, and the reconciler maps it back to the
  claim. The id returned to the model is always the one it claimed.
- A reused id inside one task is genuinely ambiguous: which call does a
  finding that cites it mean? It is reported as `duplicate-transcript` and
  `duplicate-control` with medium confidence, and findings that cite it are
  downgraded to theorized.
- `Downgrade` applies a mismatch to a finding only when the mismatch's task
  belongs to the finding's lab.

## Consequences

A multi-lab run produces no reconciliation noise from id collisions, which a
regression test covers (it fails if the join returns to per-run scope). A
model that reuses ids inside a task is still visible and its findings are
held at theorized, which is the conservative reading. The task id format
(`<lab>:<model>`) is now part of the contract between the pipeline and the
reconciler.

Amendment (2026-10-08): denied calls. When policy refuses a call, the agent
records a `policy_decision` with effect deny under the claimed id but no
`tool_call`, because nothing ran. The reconciler first reported that as
`missing-control-plane-event`, which blamed the model for a call the control
plane had in fact handled. A claim is now accounted for, and listed under
`denied` in the report, only when there is exactly one claim, no executed
call, and a deny decision in the same task for the same tool and agent. A claim
with no record at all, an allow decision with no execution, or a denial for a
different tool or in another task is still a mismatch.

