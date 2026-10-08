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
go install github.com/joeyvictorino/assay/cmd/assay@v0.1.0
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

The latest committed run is `20261007-215749-7290ba88`
([CI run](https://github.com/joeyvictorino/assay/actions/runs/37692863751/attempts/1)).
It is labelled `degraded`: the only real model was a local open-weight model
(Qwen2.5-3B, 4-bit) next to the scripted provider, because no API keys were
configured. That run does not yet show the multi-model overlap this project is
for; a run with several real models is not published.

## Status

- **Checked by deterministic code (validated or refuted):** security headers,
  verbose errors, CORS, open redirect, reflected and stored XSS, information
  disclosure, sensitive files, missing and bypassed authentication, IDOR,
  default credentials, SQL injection, path traversal, weak JWT configuration,
  mass assignment, missing rate limiting, CSRF and SSRF. Each check is
  non-destructive and needs two consistent observations.
- **Theorized only:** any finding whose class has no checker (`other`), and any
  finding a checker could not confirm.
- **Not yet shown:** a multi-model overlap result, precision and recall for a
  frontier model, and an upstream adopter of the signed-manifest design.
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
