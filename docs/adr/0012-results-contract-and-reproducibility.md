# ADR 0012: Results directory contract and reproducibility

## Status

Accepted (S4, 2026-10-07).

## Context

The project's claims are numbers: how little models overlap, how many
findings validate, what a run costs. A number in a README is unverifiable.
Several commands (`run`, `overlap`, `reconcile`, `report`, `continuous`)
and two workflows (`full-run`, `continuous`) need to agree on where a run's
outputs live and what they are called, and anyone with a clone must be able
to regenerate every derived file from the committed primary ones.

## Decision

One run is one directory `results/<YYYYMMDD>-<runid>/` holding:

- `run.json` (`model.RunReport`): the committed summary, including the
  audit head hash and record count that pin the encrypted log.
- `findings.redacted.json` (`[]model.Finding`): every finding, redacted.
- `overlap.json` (`overlap.Report`), `costs.json`, `ci.json`
  (`{run_url, run_id, attempt, sha, workflow}`).
- optional `audit.export.jsonl`: decrypted `model.AuditRecord` lines,
  digests and bounded metadata only; never bodies.
- derived: `reconcile.json`, `findings.reconciled.json`, `report.md`.

`results/LATEST` holds the newest directory name. `results/baseline.json`
is a promoted `run.json` (ADR 0013). The encrypted audit log itself is not
committed; CI keeps it as a 90-day artifact, and `run.json` commits its head
hash and count so a later export can be checked against it.

Derived files are pure functions of primary files: sorted keys, nil
containers normalized to empty, no timestamps except those copied from
`run.json`. `assay overlap --check` and byte-equality tests enforce this.
The dashboard reads only `docs/dashboard/data/index.json`, which `assay
report --format dashboard` regenerates from every `results/*/run.json`.

Results are committed by the workflow that produced them, with `[skip ci]`
and the CI run URL in the message, under a `results-commit` concurrency
group so two runs cannot interleave their pushes.

## Consequences

- Any statistic can be traced to a directory, a CI run URL and a commit.
- A reviewer can regenerate `reconcile.json` and `report.md` from the
  primary files and diff them; a difference is a bug, not noise.
- The PDF renderer and the Markdown renderer read the same files in the
  same section order, so they cannot drift apart silently.
- Committing results grows the repository by a few hundred kilobytes per
  run; a retention or squash policy is a future decision.
