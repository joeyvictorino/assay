# ADR 0003: Ed25519-signed tool manifests and a file trust store

## Status

Accepted (2026-10-07).

## Context

Models choose tools by name from the manifests they are shown. If a manifest
can be edited between review and execution, its declared risk tier,
capabilities or allowed agents can be lowered or widened without anyone
noticing, and the policy engine would reason over false inputs. The same
applies to the scope document that authorizes targets.

The harness runs in CI and on developer machines. Key material must be
simple to provision and to audit, and the verification code must not depend
on a network service.

## Decision

Every tool manifest carries `signer`, `signed_at` and `signature`. The
signature is Ed25519 over the RFC 8785 canonical bytes (ADR 0002) of the
manifest with `signature` cleared; `signer` is the first 16 hex characters of
SHA-256 over the raw public key. `internal/toolsig` implements this and
exposes the same scheme for arbitrary JSON documents (`SignDocument`,
`VerifyDocument`) so scope documents are signed identically.

The trust store is a directory of `<label>.pub` files, each a base64 raw
32-byte public key (`trust/keys/README.md`). Loading an empty directory is an
error. Verification returns one of four reason codes:
`UNSIGNED`, `SIGNATURE_INVALID`, `UNKNOWN_SIGNER`, `PROVENANCE_VALID`.

A valid signature is provenance, not authorization. The policy engine takes
the verification result as its first input and denies on anything but
`PROVENANCE_VALID`; all other checks run after that.

Keys are generated with `assay tools keygen`; the private key is written with
mode 0600 and is never committed (`*.key` is ignored). Signing is done by
`assay tools sign`, verification by `assay tools verify` with exit code 2 on
any failure, so CI can gate on it.

## Consequences

- Tampering with any manifest field after signing is detected, including
  fields the policy engine does not currently read.
- Key rotation is adding a file, re-signing, and removing the old file.
- There is no revocation protocol beyond removing a key; a removed key makes
  its manifests fail with `UNKNOWN_SIGNER`, which is the intended outcome.
- Manifests must canonicalize, so `input_schema` may not contain
  non-integer numbers.
