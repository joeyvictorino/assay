# Results

Each directory is one run: `run.json` (summary), `findings.redacted.json`
(every finding, summaries redacted), `findings.<model>.json` (per model),
`overlap.json` (cross-model matrix and precision/recall where ground truth is
exhaustive), `costs.json`, `reconcile.json`, `chains.mmd`,
`audit.export.jsonl` (decrypted audit records: digests and bounded metadata,
no bodies) and `report.md`. `ci.json` names the CI run that produced it.

Only runs with at least one real model provider are committed; fake-only
pipeline checks stay in workflow artifacts. `LATEST` names the newest run and
`baseline.json` is a promoted run used by continuous mode.

Regenerate the overlap matrix from the committed per-model findings:

```
assay overlap --inputs results/<run>/findings.<a>.json,results/<run>/findings.<b>.json --check results/<run>/overlap.json
```
