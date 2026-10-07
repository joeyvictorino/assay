# ADR 0005: Zero data retention as architecture

## Status

Accepted (2026-10-07).

## Context

The harness sends prompts to models and receives transcripts that may quote
target responses, credentials observed during probing, or source code from
the lab. A README promise not to keep transcripts is not verifiable. The
property has to be enforced by the code structure and checked by tests that
fail when it is violated.

## Decision

Zero data retention is implemented in three layers.

1. Sink. The only production implementation of `model.TranscriptSink` is
   `zdr.NullSink`, which discards its input. A retaining `MemorySink` exists
   only behind the `zdrdebug` build tag, so a release binary cannot be
   configured to keep transcripts.

2. Record shape. The audit log (ADR 0004) stores digests and bounded
   metadata. Its writer rejects any string longer than 256 bytes or matching
   a credential pattern, which makes pasting a transcript into a record
   impossible rather than discouraged. Log output passes through
   `redact.Handler` and `redact.Writer`, which replace credential-shaped
   substrings before they reach any stream.

3. Tests. `zdr.NewCanary` produces a random marker that tests plant in a
   transcript; `zdr.ScanTree` then reads every regular file under the run's
   output directory byte by byte, ciphertext included, and asserts the marker
   is absent. A `go/ast` test walks every non-test file under `internal/`
   and fails if any package other than the audit writer, report, remediation,
   results and CLI packages calls a file-creating function from `os`.

## Consequences

- Retaining a transcript requires adding code in a restricted package and
  defeating the canary test, which is visible in review.
- Debugging a model interaction needs a `zdrdebug` build; this is intended
  friction.
- The redaction pattern set is a defense in depth, not the primary control;
  the primary control is that bodies have no field to live in.
- Operators who want transcripts for their own purposes must obtain them
  from the model provider's side, not from this harness.
