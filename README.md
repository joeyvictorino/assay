# assay

A multi-model continuous security-validation harness. Routes each task to
the best model, runs several models over the same authorized targets, measures
how little their findings overlap, validates exploitability in a sandbox,
chains findings into attack paths, generates remediations, and records every
model call, tool call and decision in an encrypted append-only audit log,
with zero transcript retention by default.

## Install

Release binaries for macOS and Linux (amd64, arm64) with `checksums.txt` are on the
[releases page](https://github.com/joeyvictorino/assay/releases). Or build from source:

```
go install github.com/joeyvictorino/assay/cmd/assay@v0.3.0
```

## Quick start

The pull-request pipeline runs the whole harness against the repository's own
synthetic lab with a scripted provider, so it needs no network and no API key:

```
go build -o assay ./cmd/assay
(cd labs/synthetic-ops && go build -o ../../synthetic-ops ./cmd/synthetic-ops)
./synthetic-ops -addr 127.0.0.1:9000 &
./assay run --config runs/pr.yaml --mode full --out out --lab synthetic-ops \
  --ephemeral-audit-key --run-id demo
./assay audit verify --log .audit/demo.log
```

`assay run` refuses to start without a trust store, verified tool manifests, a
policy and a signed scope document. `scope/ci.yaml` is signed with the key in
`trust/keys`; to point it at other targets, sign your own with `assay scope sign`.

## Results

Every committed run lives under `results/<run-id>/` with the CI run that
produced it; `results/README.md` describes the files and how to regenerate the
overlap matrix. The dashboard is at
[joeyvictorino.github.io/assay/dashboard](https://joeyvictorino.github.io/assay/dashboard/).

`python3 scripts/calibration.py` prints, for every committed run with
self-reflection records, each model's own scores beside what the checkers then
said about its findings and its recall where a lab has complete ground truth.

The latest committed run is `20261010-092639-5600c802`
([CI run 38041295991](https://github.com/joeyvictorino/assay/actions/runs/38041295991/attempts/1)):
three small local open-weight models (Qwen2.5 3B, Llama 3.2 3B, Ministral 3
3B; 4-bit, on the runner's CPU) against the three labs, with no API keys and
no frontier models. All three called tools and reported findings; they
reported 9 distinct findings between them and shared none. On synthetic-ops,
the lab with complete ground truth (15 weaknesses), precision and recall were
0.667 / 0.133 for Ministral, 0.250 / 0.067 for Qwen and 0 / 0 for Llama.
Several tasks timed out on CPU. `results/README.md` has the full table, the
limits, and the two superseded ensemble runs. A run with frontier models
needs API keys and is not published.

## Status

- **Checked by deterministic code (validated or refuted):** security headers,
  verbose errors, CORS, open redirect, reflected and stored XSS, information
  disclosure, sensitive files, missing and bypassed authentication, IDOR,
  default credentials, SQL injection, path traversal, weak JWT configuration,
  mass assignment, missing rate limiting, CSRF and SSRF. Each check is
  non-destructive and needs two consistent observations.
- **Theorized only:** any finding whose class has no checker (`other`), and any
  finding a checker could not confirm.
- **Self-reflection:** each model scores its own report and revises when the
  score is below 7, at most twice (ADR 0017). The score gates revision only,
  and a model grading itself is a biased judge; the scores are recorded so
  that bias can be measured once runs with several real models exist.
- **Shown with small local models only:** a three-model overlap result and
  per-model precision and recall (`results/README.md`).
- **Not yet shown:** an overlap result or precision and recall for frontier
  models, and an upstream adopter of the signed-manifest design.
- **Zero data retention** is a tested property of this harness (an end-to-end
  test plants a canary in lab responses, model text and prompts and scans the
  disk after a run). Providers apply their own retention terms to the calls
  the harness sends; that is governed by the provider agreement and not
  claimed here.

## Principles

- Authorized targets only. A signed scope document gates every outbound
  request; the gate fails closed.
- A technical claim is not evidence. Findings start as *theorized* and
  become *validated* or *refuted* only through deterministic checks.
- Zero data retention is a tested property, not a README sentence.
- Every statistic published here is reproducible from committed code and
  committed or regenerable data.

## License

Apache-2.0. See `LICENSE`.
