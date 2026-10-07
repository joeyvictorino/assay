# ADR 0008: The authorization gate fails closed

## Status

Accepted (S3, 2026-10-07).

## Context

assay sends real HTTP requests to real services. Models propose targets, and
a model can be wrong, manipulated, or simply creative. The only acceptable
failure mode for "is this target authorized?" is "no". A scope document
describes authorization, but documents can be missing, malformed, expired,
tampered with or written by the wrong person.

## Decision

- `internal/scope` is the single implementation of `model.Gate`. It builds a
  gate only from a document that parses, carries a signature the injected
  verifier accepts, is inside its validity window, and lists at least one
  target with ports and schemes. Every other input is an error with a code
  (`SCOPE_UNSIGNED_VERIFIER_MISSING`, `SCOPE_EXPIRED`, ...), never a gate.
- Signature verification is injected (`scope.Verifier`). A nil verifier is
  itself a refusal: an unsigned or unverifiable document is never trusted,
  not even in tests.
- `Allow` matches the host exactly (`localhost` and `127.0.0.1` are different
  targets; IP literals are canonicalized so `[::ffff:127.0.0.1]` is
  `127.0.0.1`), the port explicitly (defaults 80/443 come from the URL scheme,
  never from the scope), the scheme explicitly, and the path against prefixes
  when given, refusing `..` segments and percent-encoded dots. Wildcards
  exist only as an explicit `*.`-prefixed host entry. URLs with userinfo are
  denied outright.
- `internal/httpx` is the only HTTP client. It consults the gate before each
  request, re-checks every redirect target, refuses cross-host redirects,
  strips credentials on redirect, caps bodies, and refuses to run with a nil
  gate or nil auditor. An audit write failure aborts the request.
- A per-gate token bucket (`rate_limit_per_minute`) bounds request volume;
  method allowlists bound verbs.
- The CLI never bypasses this. `--unsafe-skip-signature` exists for local
  development only, demands `ASSAY_DEV=1`, prints a banner, and records the
  key id `UNSAFE-DEV-SKIP` in every decision.

## Consequences

- A misconfigured run produces zero requests and exit code 2, not a partial
  run against whatever happened to be listed.
- Every outbound exchange has a `gate_decision` and a `tool_call` audit
  record with digests only, so "what did we touch" is answerable without
  bodies.
- Legitimate edge cases (a lab behind a redirecting proxy on another host)
  need an explicit scope entry and still cannot be followed automatically;
  the operator must target the final host directly.
- The gate cannot be unit-tested with an allow-all stub in non-test code;
  fakes live in `_test.go` files only.
