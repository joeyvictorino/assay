# ADR 0011: Attack-path chaining is deterministic

## Status

Accepted (S3, 2026-10-07).

## Context

Individual findings understate risk: a default credential plus an IDOR is
data access, not two medium issues. Models are good at narrating such chains
and bad at being consistent about them. Published results must be
reproducible from committed code and data, so the chaining step cannot be a
model call.

## Decision

- `internal/chain` derives paths with a fixed rules table (`DefaultRules`).
  A rule is `Pre -> Post`: preconditions are finding classes or facts;
  the post is a fact. Facts are a closed set: `session`, `admin-session`,
  `data-access`, `account-takeover`, `privilege-escalation`, `code-exec`,
  `config-read`, `credential-exposure`. Four of them are objectives.
- Only `validated` and `theorized` findings participate; `refuted` and
  `declined` never chain. Findings are grouped by lab and sorted by id; the
  rules run to a fixpoint; for each fact the best-confidence derivation wins,
  ties resolved by table order. The result is one path per (lab, objective).
- A path is a linear `[]model.PathStep` in the orbit-ir trajectory style:
  findings in dependency order, then facts, then the objective, each step
  typed (`finding | fact | objective`). `Confidence` is the minimum over the
  findings on the path (`validated > theorized`). `SourceRefs` lists the
  finding ids and the audit references (`audit:<run>:<seq>`) their
  evidence cites, sorted and deduplicated. The path id is a hash of lab,
  objective and finding ids.
- `Mermaid` renders paths as `flowchart LR` with finding ids as node ids,
  facts as stadium nodes and objectives as double circles; nodes and edges
  are emitted sorted and deduplicated so the diagram is byte-stable.
- The rules table is data. Changing it is a reviewed commit, covered by a
  test that every pre/post names a known class or fact and that no rule
  derives its own precondition.

## Consequences

- The same findings file always yields the same paths and the same diagram,
  on any machine, which is what `results/` requires.
- The table encodes judgment (for example that reflected XSS can yield a
  session). It is deliberately conservative and will miss creative chains a
  model might describe; those belong in model output as `theorized`
  narrative, not in `RunReport.Chains`.
- One path per objective hides alternative routes. If reports need them, the
  builder can enumerate derivations without changing the data model.
- Paths inherit theorized confidence from any unvalidated step, so a chain is
  only as strong as its weakest checker.
