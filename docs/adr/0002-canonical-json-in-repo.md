# ADR 0002: Canonical JSON implemented in the repository

## Status

Accepted (2026-10-07).

## Context

Several components hash or sign JSON: tool manifests (ADR 0003), scope
documents, audit records (ADR 0004), policy decisions and tool arguments
(`args_digest`). A hash is only useful if every party computes it over the
same bytes. Go's `encoding/json` output is stable for a given Go version but
is not a specification: it escapes HTML characters, its map ordering is by Go
string comparison, and its float formatting is implementation-defined.

RFC 8785 (JSON Canonicalization Scheme) specifies a byte-exact encoding:
members sorted by UTF-16 code units, minimal string escaping, no whitespace,
and ES6 number formatting. The number rules are the hard part; matching the
shortest round-trip float formatting of ES6 across languages is error-prone.

No third-party dependency is wanted for this (the module allows one YAML
library and nothing else).

## Decision

`internal/canon` implements RFC 8785 for objects, arrays, strings, booleans,
null and integers. Non-integer numbers are rejected with `canon.ErrFloat`.
Whole-valued floats within 2^53 are accepted and emitted as integers.

Callers that need fractional quantities store integers in a fixed unit
(`cost_microusd`, `latency_ms`) rather than floats. The policy file hash is
taken over the file bytes, not the canonical form, because budgets in the
file are human-written decimals.

Structs are supported by a round trip through `encoding/json` with
`UseNumber`, so struct tags apply and large integers are preserved as text.

`canon.Hash` is hex SHA-256 over `canon.Bytes`. Every digest in the system
that is not explicitly a file hash is a `canon.Hash`.

## Consequences

- Any component in any language can recompute every digest from the RFC
  alone, as long as it also refuses floats.
- The RFC 8785 appendix vectors (minus the float array) are unit tests.
- A record containing a float fails to write. This is deliberate: it
  surfaces an accounting bug at the source instead of producing a digest
  that another implementation cannot reproduce.
- Strings must be valid UTF-8; invalid input is an error, not replaced.
