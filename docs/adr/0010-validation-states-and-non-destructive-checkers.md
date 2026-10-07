# ADR 0010: Validation states and non-destructive checkers

## Status

Accepted (S3, 2026-10-07).

## Context

A model saying "this endpoint is injectable" is a claim, not evidence. assay
publishes precision numbers, so every finding needs an evidentiary state that
a deterministic process assigned. At the same time the checkers run against
shared lab containers in CI and, under a signed scope, against real services;
they must never damage, lock out, exhaust or persist anything.

## Decision

- Findings carry one of four states (`model.FindingState`): `theorized`
  (reported, unchecked), `validated` (a checker confirmed it), `refuted` (a
  checker disproved it), `declined` (the model refused; no content).
- `internal/validate.Validate` is the only path from `theorized` to
  `validated` or `refuted`. A class with no checker, a checker error, a gate
  denial or an inconclusive result leaves the finding `theorized`. Nothing
  else can mark a finding `validated`.
- `validated` requires two consistent observations (`MinObservations`), each
  backed by a digest-only `Evidence` record that cites the audit sequence of
  the exchange. `refuted` requires the same number of consistent negative
  observations; mixed results are inconclusive.
- Checkers are read-only by construction:
  - SQLi uses a boolean differential (tautology versus contradiction in
    quoted, LIKE and numeric forms) and compares result counts. No stacked
    statements, no DDL/DML, no time-based functions; a test asserts the
    payload table contains none.
  - Reflected XSS reflects `<b>assay-<hex></b>`, never a script.
  - Open redirect asks for a redirect to `assay-<hex>.invalid` and reads the
    `Location` header without following it (`DoNoFollow`; `.invalid` cannot
    resolve).
  - Path traversal reads the lab's own README marker via `../`.
  - IDOR logs in as two lab accounts and reads each other's record; it never
    writes. Default credentials attempt the documented login twice; the
    password never appears in evidence or notes.
  - auth-missing validates only paths the ground truth marks
    `auth_required`; info-disclosure and sensitive-file require a planted
    marker, so a real secret is never needed to prove disclosure.
- Every request goes through `internal/httpx` and therefore through the gate
  and the audit log.

## Consequences

- Precision and recall are computed over states a human did not assign and a
  model could not influence after the fact.
- Classes without a checker (CSRF, JWT, SSRF, mass assignment, rate limit)
  can only ever be `theorized` until someone writes a non-destructive check;
  that is a visible gap in reports rather than a silent overclaim.
- Checkers cost at least two requests each; the gate's rate limit bounds the
  total.
- Lab-specific prerequisites (two accounts, a login endpoint, markers) live in
  the lab adapter, so adding a lab means implementing `labs.Lab` and, where
  sessions matter, `labs.Authenticator`.
