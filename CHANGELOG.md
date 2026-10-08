# Changelog

## 0.2.0 (2026-10-08)

Added
- Seven more deterministic checkers: authentication bypass (with a control
  request so a catch-all page is not read as a bypass), weak JWT
  configuration (offline default-secret check and alg:none), mass assignment
  (benign probe field; a privileged field is never written), stored XSS,
  missing rate limiting (16 paced requests, observational), CSRF and SSRF
  (observation only; SSRF only ever points at the lab's own loopback address).
  Every class except `other` now has a checker.
- Wave orchestration with dependencies, retry once, self-reflection scoring
  (a score below 7 triggers at most two revisions), and delegation limited to
  depth 3 with cycle refusal (`internal/orchestrate`, `assay plan`, ADR 0015).
- Eval cases as regression bands with advisory targets, `assay eval`, and an
  evals workflow that opens an issue on a regression (ADR 0016).
- `assay scope sign` and `assay scope verify`; a signed CI scope document.
- An end-to-end retention test that plants a canary in lab responses, model
  text and prompt text and scans the sandboxed disk after a run, with a
  positive control for the scanner.
- A dashboard on GitHub Pages and a recorded terminal demo.
- A draft proposal for signed tool definitions in MCP (`docs/proposals`; not
  submitted anywhere).

Fixed
- Reconciliation joined tool call ids over the whole run, but ids are only
  unique within one task. A scripted provider reusing ids across labs produced
  false duplicate-id mismatches that downgraded its findings. The join is now
  per task, and a two-lab test fails if it regresses (ADR 0014).
- The scheduled `continuous` workflow failed because it and the workflow it
  calls held the same concurrency group.
- The DVWA setup sent the wrong cookie jar; the local model's context window
  was too small for Juice Shop.

Known limitations
- The orchestrator is not yet called by `assay run`; self-reflection scoring
  is library code with tests, not part of the pipeline.
- No run with frontier-model providers is published. The committed runs are
  degraded: one local 3B model plus a scripted provider. The local model's
  output varies between runs (validated findings on the synthetic lab: 3, 0, 1
  in three runs), so its eval case is advisory.
- A validated finding means a deterministic check reproduced the behavior in
  an authorized lab, nothing more.

## 0.1.0 (2026-10-07)

First release: provider router with failover and budgets, Ed25519-signed tool
manifests with a trust store, a deny-wins policy engine, an AES-256-GCM
hash-chained audit log, a fail-closed scope gate, a gated HTTP client, twelve
deterministic checkers, attack-path chaining, remediation templates,
reconciliation of claimed against executed tool calls, cross-model overlap
measurement, and a CI workflow that runs the harness against three labs and
commits the result with the CI run that produced it.
