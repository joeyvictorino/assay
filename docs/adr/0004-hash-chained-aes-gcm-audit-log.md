# ADR 0004: Hash-chained, AES-GCM encrypted audit log

## Status

Accepted (2026-10-07).

## Context

Every published statistic in this project is reproduced from what the
harness recorded: model calls, tool calls, policy and gate decisions,
findings and scores. The record must therefore be tamper-evident, and it
must be verifiable by someone who holds only the file, as well as by someone
who also holds the key. At the same time the record must not become a copy
of the transcripts it describes (ADR 0005).

## Decision

`internal/audit` writes one JSON-lines file per run.

- Line 1 is a header: format version, run id, key id, start time.
- Each following line is `{"seq","prev","nonce","ct","hash"}` where
  `hash = sha256(prev || nonce || ct)` and `prev` is the previous line's
  hash. The first `prev` is `sha256("assay-genesis:" + run_id)`.
- `ct` is AES-256-GCM over the canonical bytes (ADR 0002) of the record,
  with `{"seq","prev","run_id"}` as additional authenticated data, so a
  record cannot be moved to another position or another run.
- The key is derived per run: HKDF-SHA256(master, salt = run id,
  info = "assay-audit-v1"). The key id in the header is the first 16 hex
  characters of SHA-256 over the derived key, which lets a verifier detect a
  wrong key before decrypting anything.
- The file is opened append-only with mode 0600. Reopening an existing file
  verifies the whole chain first and refuses to continue a broken one.

Records carry digests and bounded metadata only. The writer rejects any
record whose string values exceed 256 bytes or match the secret patterns in
`internal/redact`; keyed verification repeats that check.

`Verify` without a key checks the header, every line hash and the chain, and
detects nonce reuse. With a key it also decrypts each record, checks the
inner sequence, previous hash and run id against the line, and counts
records by kind. Errors name the kind (`ErrChainBreak`, `ErrHashMismatch`,
`ErrWrongKey`, `ErrTruncated`) and the line number.

## Consequences

- A single flipped byte, a deleted line, a reordered line or a cut-off tail
  is detected, each with a distinct error and a line number.
- Removing the last line is not detectable from the file alone; the run
  report commits the head hash and record count for that purpose.
- The master key lives only in an environment variable named on the command
  line; it is never a flag or a file in the repository.
- Records cannot contain non-integer numbers; costs are in micro-units.
- Random 96-bit nonces are used; reuse is detected at verification time.
