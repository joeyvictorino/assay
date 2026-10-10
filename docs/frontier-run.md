# Running assay against frontier models

Today there is no frontier-model result. Every committed run used small local
open-weight models, because the repository has no provider API keys. Adding
the keys and dispatching one workflow produces the first one. This page is the
whole procedure.

## What you do

1. Store the keys as repository secrets. `gh` prompts for the value, so it does
   not land in your shell history:

   ```
   gh secret set ANTHROPIC_API_KEY --repo joeyvictorino/assay
   gh secret set OPENAI_API_KEY    --repo joeyvictorino/assay   # optional
   gh secret list --repo joeyvictorino/assay                    # ASSAY_AUDIT_KEY is already there
   ```

   An Anthropic key alone is enough. Without `OPENAI_API_KEY` the OpenAI model
   is dropped, the run is labelled `degraded` (meaning "a configured provider
   had no key"), and you can skip step 2.

2. Only if you set `OPENAI_API_KEY`: choose the OpenAI model. `runs/frontier.yaml`
   ships with the placeholder `REPLACE_WITH_OPENAI_MODEL_ID` in three places
   (`models`, `routes.probe`, `cost_per_mtok`). Use a chat model that supports
   tool calls, and replace the placeholder prices with the vendor's current ones:

   ```
   sed -i '' 's/REPLACE_WITH_OPENAI_MODEL_ID/<model-id>/g' runs/frontier.yaml   # on Linux: sed -i
   $EDITOR runs/frontier.yaml          # set input_usd, output_usd, cache_read_usd for that model
   go test ./internal/config/ ./internal/cli/
   git commit -am "frontier: set the OpenAI model id and prices" && git push
   ```

   Pushing does not start a run (see "Push-triggered runs" below). If you set the
   key and leave the placeholder, `assay run` refuses to start before any API
   call and says so.

3. Dispatch the run and watch it:

   ```
   gh workflow run full-run.yml --repo joeyvictorino/assay -f config=frontier
   gh run list --repo joeyvictorino/assay --workflow full-run.yml --limit 1   # note the run id
   gh run watch <run-id> --repo joeyvictorino/assay --exit-status
   ```

   The workflow fails within seconds, before building anything, if
   `ANTHROPIC_API_KEY` is not set.

That is all. The run assesses each lab with each model independently, validates
the findings with the deterministic checkers, computes the overlap and
precision/recall, and commits `results/<run-id>/` and the dashboard data to
`main` (commit message `results: <run-id> [skip ci]`). `git pull` brings it down.

## Models

| Config entry | Model id | Price per million tokens (input / output / cache read) |
| --- | --- | --- |
| Anthropic | `claude-opus-5-5` | $4 / $20 / $0.20 |
| Anthropic | `claude-sonnet-5-5` | $2 / $10 / $0.20 |
| Anthropic | `claude-haiku-4-5` | $1 / $5 / $0.10 |
| OpenAI | your choice, in `runs/frontier.yaml` | yours to fill in |

Anthropic ids must not carry a date suffix; the config validator rejects one.
The Anthropic prices are the table already in `runs/ci.yaml`.

`runs/ci.yaml` (the default for `-f config=ci`) is a smaller mix: Sonnet 5.5 and
Haiku 4.5 when `ANTHROPIC_API_KEY` is set, plus the pinned local model. Use
`frontier` for the real comparison.

## Cost

The cap is **$15 per run** (`budgets.per_run` in `runs/frontier.yaml`), with $5
per model and $1.50 per agent loop. The run and model caps are held across all
labs and models: before each assessment starts, and before every model call,
the harness checks what is left. One call that is already in flight can overshoot
by its own cost. A run that reaches the cap is reported `BLOCKED`, its results
are uploaded as a workflow artifact, the job fails, and nothing is committed.

What is not known: the real cost. No frontier run has happened, so there is no
measurement. For scale only, the heaviest local model used 16,323 fresh input,
83,300 cached-read and 4,368 output tokens across all three labs
(`results/20261010-092639-5600c802/costs.json`), which priced at Opus 5.5 rates is
about $0.17. Frontier models will take more turns and write longer reports, so
expect more, and treat the $15 cap as the only guarantee. Costs are computed
from the table in the config and recorded in `run.json` and `costs.json`;
cache-write tokens are priced at the input rate although Anthropic charges a
premium for them, so the invoice is authoritative.

To spend less on a first attempt, lower `per_run` (for example to 5, with
`per_model` 2 and `per_task` 1.5; the validator requires per_task <= per_model
<= per_run) before dispatching. If it is reached you get a `BLOCKED` artifact,
not a committed result. Setting a spend limit in each provider's console as well
is sensible; the harness cap applies per run only.

## What to check afterwards

- `gh run view <run-id>`: the job should be green. A red job with
  `no completed model call for: <model>` means that model never answered
  (bad key, wrong model id, outage); nothing was committed.
- `results/LATEST` names the new run. In its `run.json`: `config` is
  `frontier`, `truncated` is `false`, every model under `models` has non-zero
  `usage.input_tokens`, and `spent_usd` is under 15. `mode: "degraded"` is
  expected when `OPENAI_API_KEY` was not set.
- `python3 scripts/check_overlap.py results/<run-id> --assay ./assay` recomputes
  the overlap matrix from the committed findings (CI does this for every run).
- The `evals` workflow runs after the run and opens or closes an "Eval
  regression" issue.
- Then update the "Latest committed result" section of `README.md` and the
  Results section of `results/README.md` with numbers copied from `run.json`.

## Things that may go wrong

- The Anthropic and OpenAI adapters have been tested against local stub servers
  only, never a live API. If the first live calls fail (for example every call
  returns HTTP 400), the run finishes with no findings from that model and the
  commit gate refuses it; the job log prints the error for each model and lab
  (`run: <model> on <lab>: ...`). Requests rejected with a 400 are not billed
  for tokens.
- Opus 5.5 and Sonnet 5.5 think by default, and thinking tokens count against
  `agent.max_tokens`. `runs/frontier.yaml` sets 8192 (the local runs used 2048),
  so the two result sets do not share every limit. Say so when comparing them.
- Refusals are recorded and never retried (ADR 0009). A model that declines a lab
  contributes zero findings for it.

## Push-triggered runs

`full-run.yml` used to start on any push to `main` that touched `internal/`,
`labs/`, `runs/` or `scope/`. Without keys that produced a degraded run that
replaced the latest result, and with keys it would have spent money on every
code change. A push now starts the run only when the repository variable
`ASSAY_FULL_RUN_ON_PUSH` is `true`, which is not set. Scheduled runs
(`continuous.yml`) and manual dispatch behave as before, except that
`scripts/ci/results-gate.py` commits a degraded run only when it was dispatched
by hand and never commits a run in which a real model made no completed call.
