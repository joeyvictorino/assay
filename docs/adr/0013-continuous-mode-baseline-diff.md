# ADR 0013: Continuous mode is a diff against a promoted baseline

## Status

Accepted (S4, 2026-10-07).

## Context

A single run shows what models found today. The interesting question over
time is drift: a finding that appears, one that stops reproducing, one a
checker previously refuted and now confirms, a model whose cost or latency
moves. Model APIs change under us without notice, so this has to run on a
schedule and raise a visible signal without a person watching.

## Decision

- `results/baseline.json` is a `run.json` a human promoted, with the
  matching findings in `results/baseline.findings.json`. Promotion is an
  explicit `workflow_dispatch` input (`promote_baseline`), never automatic.
- `internal/continuous.Diff` compares the baseline with the latest run by
  dedup key (ADR 0007). A key is *active* when its state is `theorized` or
  `validated`. `New` = active now and absent from the baseline. `Fixed` =
  active in the baseline and absent or `refuted` now. `Regressed` =
  `refuted` in the baseline and active now, or active in both with a higher
  severity now. `refuted` and `declined` findings never count as new.
  Cost delta is `spent_usd` difference; p95 deltas cover models present in
  both runs.
- `Verdict` returns 1 when any new or regressed key has severity at or
  above `--fail-on` (default `high`); fixed findings never fail.
- `.github/workflows/continuous.yml` runs at 06:00 UTC on Mondays and
  Thursdays, calls `full-run`, then `assay continuous diff`. On exit 1 it
  opens or updates one issue titled "Continuous validation drift" with the
  Markdown body and closes it when a later run is clean. No baseline means
  no diff and a notice, not a failure.

## Consequences

- Drift is a GitHub issue with dedup keys, not a number someone has to
  remember to compare.
- Confirming a theorized finding (`theorized` to `validated`) is neither
  new nor regressed; it is the harness doing its job.
- A model that is dropped for a missing key appears as fixed findings in a
  degraded run. The issue body shows the run id and mode so this is
  readable, but the threshold does not distinguish it; promoting a degraded
  run as baseline is a human choice to avoid.
- Severity comes from the current run's findings first, so a model that
  re-rates an old finding can trip the threshold; that is reported as a
  regression by design.
