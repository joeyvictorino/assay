# Contributing to assay

Thank you for your interest.

## Development

- Go 1.26
- `gofmt -l .` must print nothing; `go vet ./...` must be clean
- `go test ./... -count=1 -race` from the repository root
- `sh scripts/name-audit.sh` before every push
- Standard library plus `gopkg.in/yaml.v3` only in the root module; the
  Anthropic SDK is the one provider exception. The lab module under
  `labs/synthetic-ops` keeps its own dependencies and never leaks into the
  root module (`GOWORK=off go build ./cmd/assay` must work).

Quick checks:

```sh
go run ./cmd/assay version
go run ./cmd/assay tools verify --manifests internal/tools/manifests --trust trust/keys
go run ./cmd/assay policy eval --policy policy/default.yaml --manifests internal/tools/manifests \
  --trust trust/keys --agent recon --tool http_get
```

## Ground rules

1. **Authorized targets only.** Nothing may send a request that did not pass
   the signed scope gate. Do not add code paths around `internal/httpx`.
2. **No detection evasion.** Rate limiting, method allowlists and plain
   requests are features. Pull requests adding obfuscation, fingerprint
   rotation or WAF bypass are declined.
3. **Refusals stand.** Never retry, rephrase or re-route a declined model
   request.
4. **No transcript retention.** New code may not write model text to disk
   or to the audit log. The `zdr` canary and AST tests enforce this; do not
   widen their allowlists without an ADR.
5. **Checkers are read-only.** A new validator may not write, delete, lock
   out, exhaust or persist anything on a target, and must produce
   digest-only evidence that cites audit sequence numbers.
6. **Determinism.** Anything that lands under `results/` must be
   byte-reproducible from committed inputs: sort maps, normalize nil
   containers, no timestamps in derived files.

## Adding a checker

1. Implement it in `internal/validate` behind the class it covers.
2. Add a table-driven test with an `httptest` server; include a case that
   proves the payload table has no destructive statement.
3. Plant any marker it needs in the lab (`labs/synthetic-ops`) and list the
   ground truth entry.
4. Update ADR 0010 if the evidentiary rule changes.

## Adding a lab

Implement `labs.Lab` (and `labs.Authenticator` when sessions matter), add a
`ground_truth.json`, a loopback-only CI service in
`.github/workflows/full-run.yml`, and a wait step. Labs are deliberately
vulnerable software: bind to loopback, document that fact at the top of the
README, and never reuse their handlers elsewhere.

## Tool manifests, policy, scope

Manifests under `internal/tools/manifests` must be signed by a key in
`trust/keys`; CI runs `assay tools verify`. Policy changes go through
`assay policy eval` cases in the tests. Scope documents in `scope/` are
examples; a real scope is signed by the person who authorized the target.

## Reporting issues

Use the issue templates. Include:

- `assay version` (or the commit)
- The subcommand and flags
- The relevant results directory name under `results/`, if any
- Redacted output; never a scope document, an audit key or a provider key

## Code of conduct

Be respectful. Keep the discussion on the problem: measuring what models
actually find, with evidence, against targets we are allowed to test.
