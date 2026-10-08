# ADR 0016: Evals are regression bands, not absolutes

Status: accepted (2026-10-07)

## Context

Model behavior varies between runs, so a fixed pass mark either flaps or is
set so low that it proves nothing. What a maintainer needs is a signal that a
change made the harness worse than it was.

## Decision

`internal/evals` loads cases from `evals/cases/*.yaml`. A case names a lab and
a model glob and sets bands: minimum validated findings, a ceiling on the
share of theorized findings, cost, p95 latency, reconciliation mismatches,
and whether any refusal is acceptable. `evals/thresholds.yaml` holds the
defaults. `assay eval --run DIR` evaluates the newest run and exits 0 on pass,
1 on a band miss and 2 on an error.

Cases marked `advisory: true` document a target that is not yet met. They
appear in the report and never fail the verdict. The workflow
`.github/workflows/evals.yml` runs the evaluation when a results commit lands
and opens or updates one issue titled "Eval regression" on a miss.

## Consequences

Bands are set from committed runs, so each one is traceable to evidence.
Tightening a band after an improvement is a reviewed change to a YAML file.
A band proves the harness did not regress on the labs it covers; it says
nothing about targets outside them. The local model's zero precision on the
synthetic lab is recorded as an advisory case rather than hidden.
