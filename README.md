# assay

A multi-model continuous security-validation harness. Routes each task to
the best model, runs several models over the same authorized targets, measures
how little their findings overlap, validates exploitability in a sandbox,
chains findings into attack paths, generates remediations, and records every
model call, tool call and decision in an encrypted append-only audit log,
with zero transcript retention by default.

**Status: pre-release scaffold.** Nothing here is a result yet. Results are
published only under `results/` with the CI run that produced them and a
script that regenerates them.

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
