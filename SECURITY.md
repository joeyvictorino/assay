# Security Policy

## Supported Versions

Only the latest release is supported.

## Reporting a Vulnerability

If you believe you have found a vulnerability in assay itself (not in the
lab services it targets), open a private security advisory on GitHub or
email the maintainer. Include the version (`assay version` or the commit),
the subcommand and flags, and the smallest input that reproduces the
problem. Do not include scope documents, audit keys or provider keys.

Do not file public issues for suspected security problems in the tool.

## What assay does and does not claim

assay is a security-validation harness that drives language models against
deliberately vulnerable services and records what happened. Explicitly:

- **assay only touches authorized targets.** Every outbound request passes
  the signed scope gate (`internal/scope`, ADR 0008). A target that is not
  listed in a signed, unexpired scope document is denied, and the gate
  fails closed: a missing, malformed, expired or unverifiable scope produces
  zero requests and exit code 2. The gate cannot be disabled in CI; the
  development override demands `ASSAY_DEV=1`, prints a banner and marks
  every decision it made.
- **assay does not evade detection.** It sends plain requests through one
  gated HTTP client with a rate limit and a method allowlist. There is no
  traffic shaping, user-agent rotation, encoding obfuscation or WAF bypass
  logic, and contributions adding any are declined.
- **assay does not work around model refusals.** A declined request is
  recorded and never retried, rephrased or routed elsewhere (ADR 0009).
- **assay retains no transcripts by default.** The only production
  transcript sink discards its input; the audit log stores digests and
  bounded metadata and its writer rejects anything longer or
  credential-shaped (ADR 0005). A debug sink exists only behind a build tag.
- **Provider-side retention is not claimed.** What a model provider keeps
  is governed by that provider's agreement with the operator. assay
  measures and tests its own retention, nothing upstream of it.
- **A `validated` finding means one thing.** A deterministic, read-only
  checker reproduced the behavior in an authorized lab, with digest-only
  evidence that cites the audit log (ADR 0010). It does not mean the issue
  is exploitable in production, that a model "found" it unaided, or that a
  clean run proves anything secure.
- **assay never modifies a target.** Checkers are non-destructive by
  construction (ADR 0010); remediations are drafted into the results
  directory, never applied to a target.
- **Published numbers are reproducible.** Everything under `results/` is
  regenerated from committed code and committed or regenerable data by the
  CI run it names.

## Keys and secrets

The audit master key and provider API keys are read from environment
variables named on the command line and never from flags or files in the
repository. Signing keys for tool manifests and scope documents are never
committed (`*.key` is ignored). The committed trust store holds public keys
only.
