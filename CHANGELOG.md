# Changelog

## Unreleased

Added
- `scripts/demo.sh`: a 60-second offline demo (no API key, no model) that builds
  the harness and the synthetic lab, runs the scripted fake provider against it,
  prints a plain-English summary and shows the audit chain failing after an edit.
  `docs/demo.tape` records it.
- `runs/frontier.yaml` and `docs/frontier-run.md`: a frontier run (Claude Opus
  5.5, Sonnet 5.5, Haiku 4.5, and an OpenAI model whose id is a placeholder to
  fill in), dispatched with `gh workflow run full-run.yml -f config=frontier`.
  Capped at $15 per run.
- `scripts/ci/results-gate.py`: decides what the full-run workflow may commit.

Fixed
- The per-run and per-model USD caps applied to each (lab, model) assessment
  separately, because every assessment builds its own router. They now hold
  across the whole run, and a run that reaches one is BLOCKED (ADR 0006
  amendment).
- `runs/ci.yaml` routed every task kind to the fake provider and listed the fake
  model among the assessed models, so a keyed run would have included a
  scripted model in the overlap matrix. Routes now point at the Anthropic models
  with the local model as the keyless fallback.
- The signed `delegate` tool listed `allowed_agents` [recon, probe], but
  `policy/default.yaml` denies delegation to those agents and allows it only for
  the orchestrator. The manifest check runs first, so the orchestrator was
  refused (`AGENT_NOT_ALLOWED_FOR_TOOL`) and the allow rule was unreachable. The
  manifest now lists [orchestrator], re-signed with the owner key already in
  `trust/keys`, and tests evaluate the shipped manifests against the shipped
  policy. A side effect: probe and recon agents are no longer offered a tool
  they could never use.
- A push to `main` that touched `internal/`, `labs/`, `runs/` or `scope/`
  started a full run whose degraded result replaced the latest one. Push-triggered
  runs are now opt-in (repository variable `ASSAY_FULL_RUN_ON_PUSH`), a degraded
  run is committed only when dispatched by hand, and a run that hit its budget cap
  is uploaded as an artifact instead of being lost.

Known limitations
- `assay run` does not wire delegation: `tools.Env.Delegate` is never set, so a
  `delegate` call would return "delegation not available in this context". The
  tool is now authorized for the orchestrator by manifest and policy, which is
  tested, but multi-agent delegation does not run in the pipeline.
  `internal/orchestrate` has it as library code with tests.
- Four more manifest and policy disagreements remain: the policy lists
  `report_finding` and `score_task` for the orchestrator, and `http_get` and
  `report_finding` for the remediate agent, but those manifests' `allowed_agents`
  exclude those agents, so the requests are denied. The default pipeline does
  not use either agent for those tools.

## 0.3.0 (2026-10-08)

Added
- Self-reflection in the pipeline (ADR 0017): `assay run` has each model score
  its own report from 0 to 10 through the signed `score_task` tool and sends it
  back with its own critique when the score is below 7, at most twice. The run
  report and `report.md` record the scores and revision counts per lab and
  model. A score gates revision only; findings are validated by deterministic
  checkers alone. Critique text is not stored.

Fixed
- The evals workflow never ran: it listened for pushes touching `results/`,
  but results commits are marked `[skip ci]`. It now listens for the `full-run`
  and `continuous` workflows finishing.
- The local model's context window was too small for Juice Shop during a
  revision; it is now 32768 tokens. A revision that fails is recorded in the
  run report's reflection note instead of being silent.
- The latency band now describes a lab assessment including reflection.

- Reconciliation reported a claimed call that policy had denied as a missing
  control-plane event. Denied calls are now listed under `denied` and not
  counted as mismatches; a claim with no record, or an allow with no execution,
  is still a mismatch (ADR 0014).
- A lab assessment that fails entirely is now recorded in the run report's
  reflection note instead of showing blanks.

Known limitations
- The local 3B model cannot finish Juice Shop on the CI runner: with a 32k
  context its calls exceed the 10-minute per-call timeout, so that lab has no
  local-model findings or reflection record. This is a measured limit of the
  model and runner, not something the harness hides.
- The local model's results vary a lot between runs (validated findings on the
  committed runs: 2, then 6, then 0), so no per-model quality claim is made
  from them.

Changed
- Known limitation from 0.2.0 closed: the orchestrator is now called by
  `assay run`. Three committed runs (a local 3B model plus a scripted
  provider) include reflection data. They show the mechanism works end to end;
  they do not show that revision helps or that a model's self-score tracks its
  results.

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
