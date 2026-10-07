# ADR 0001: Architecture overview

Status: accepted (2026-10-07)

## Context

The harness must let several models assess the same authorized targets,
measure how little their findings overlap, and produce results a reader can
reproduce from the repository alone. The owner's standard is that a technical
claim is not evidence, so every component is built to make its own output
checkable.

## Decision

One Go module with small packages, each owning one invariant:

- `scope` and `httpx`: a signed scope document gates every outbound request;
  the client is the only path to the network.
- `toolsig`, `policy`: tools are Ed25519-signed manifests verified against a
  trust store; a deny-wins policy decides per agent, per tool, per tier.
- `provider`, `router`, `agent`: pluggable model backends, task-to-model
  routing with failover and budgets, and a per-model agent loop whose only
  way to report is the `report_finding` tool with a closed class list.
- `finding`, `overlap`: deterministic dedup keys so "the same finding" is a
  property of the request shape, not of a model's prose.
- `validate`, `chain`, `remediate`: deterministic checkers set state;
  chaining and remediation are rule tables, not model output.
- `audit`, `redact`, `zdr`, `reconcile`: an encrypted hash-chained log of
  every model call, tool call and decision; nothing retained but digests and
  bounded metadata; model-claimed tool calls reconciled against executed ones.
- `labs`: adapters for intentionally vulnerable training applications plus a
  repo-owned synthetic service with complete ground truth.
- `report`, `continuous`: results written as a fixed file contract and
  diffed against a promoted baseline.

Full runs execute in CI with service containers; a committed result always
names the CI run that produced it.

## Consequences

Each invariant is testable in isolation and the lead integrates them in one
`run` command. The price is more packages than a small tool would need and
some duplicated small types across module boundaries (the lab module cannot
import the root). Subsequent ADRs record the individual decisions.
