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

Recompute a run's overlap matrix from its committed per-model findings and
compare every matrix field with the committed `overlap.json` (CI does this for
every committed run):

```
go build -o assay ./cmd/assay
python3 scripts/check_overlap.py results/<run>
```

The script wraps `assay overlap --inputs <findings files> --out FILE`. It
does not compare precision: the run scores each model per ground-truth lab,
while `assay overlap --ground-truth` scores all of a model's findings against
one lab's list. Precision is in `run.json` and `report.md`.

## Local open-weight ensemble

`runs/local-ensemble.yaml` runs three small open-weight models side by side
on the CI runner's CPU, with no API keys and no frontier models:
Qwen2.5 3B Instruct, Llama 3.2 3B Instruct and Ministral 3 3B Instruct, all
Q4_K_M GGUF files pinned by sha256 in `labs/ensemble/`. Dispatch it with
`gh workflow run full-run.yml -f config=local-ensemble`. Its `run.json` carries
`"config": "local-ensemble"` and a label saying what ran.

`20261010-092639-5600c802`
([CI run 38041295991](https://github.com/joeyvictorino/assay/actions/runs/38041295991/attempts/1))
is the first run in which all three models called tools and reported
findings. Synthetic-ops is the only lab with complete ground truth (15
weaknesses):

| Model | Findings (validated / theorized) | TP | FP | FN | Precision | Recall |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Ministral 3 3B | 2 / 1 | 2 | 1 | 13 | 0.667 | 0.133 |
| Qwen2.5 3B | 1 / 4 | 1 | 3 | 14 | 0.250 | 0.067 |
| Llama 3.2 3B | 0 / 1 | 0 | 1 | 15 | 0.000 | 0.000 |

Across all three labs the models reported 9 distinct findings (by dedup
key) and shared none: union 9, intersection 0, Jaccard 0.000 for every pair.
Ministral had 3 findings no other model reported, Qwen 5 and Llama 1.

Read it as what it is: three 3B-class models at 4-bit on four CPU cores, eight
turns each. Several tasks did not finish. Qwen and Ministral ran out of time
on juice-shop ("context deadline exceeded") and Ministral also on dvwa, one
Ministral juice-shop request needed 73,374 tokens against a 32,768-token
context, and llama-server rejected some Llama 3.2 outputs as unparsable tool
calls. It shows the harness attributing, validating and comparing findings
across three model families with every number recomputable. It is not a
finding about frontier models and says little about which small model is
better.

Two earlier ensemble runs are committed and superseded:
`20261010-072327-3b2f2823` used Granite 3.3 2B, which never called a tool
(llama.cpp left its calls unparsed), and a 16,384-token context that one
prompt exceeded. `20261010-081616-83b2a52b` hit a harness bug, fixed in
ebfe347: tool results were sent after an empty user message, which strict
chat templates reject, so Ministral failed every task.
