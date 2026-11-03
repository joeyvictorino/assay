# ADR 0019: A revision that changes nothing ends reflection

Status: accepted (2026-11-01)

## Context

ADR 0015 bounds reflection by a pass score of 7 and at most two revisions.
Within those bounds a model that cannot improve its report is sent back a
second time with the same critique and produces the same report, which is
then scored again. The second scoring call costs money and tells the run
nothing it did not know; the report is byte-identical to the one already
scored. Orchestration loops that work in practice stop when the manager
reissues the task it already issued, because a reissue means the loop has
converged, not that another turn will help.

## Decision

In `internal/orchestrate`, when a revision returns an output identical to the
output it was asked to improve, reflection stops:

- The revision attempt is recorded with `no_progress: true` and no error. The
  task stays succeeded with the last scored output and the last score.
- The identical output is not scored again. An audit record of kind `score`
  with `no_progress: true` and no score marks the stop.
- `assay run` writes the reflection note "revision added nothing new;
  stopped" in the run report. The revision still counts as a revision.

The pass score and the cap of two revisions from ADR 0015 are unchanged; this
adds a third way for reflection to end. A revision that adds or changes any
finding is still scored and may still trigger the second revision.

## Consequences

A run spends at most one wasted revision on a model that has nothing to add,
and never a wasted scoring call. The cost of a lab assessment with reflection
is bounded more tightly than before.

A model that revises by rephrasing cannot trip this: the output compared is
the harness's own summary of findings by dedup key, not model prose, so the
comparison is about what was reported and not how it was said.

The counts in `ReflectionResult.Revisions` remain comparable across runs made
before and after this decision; only the number of scores per record may be
lower.
