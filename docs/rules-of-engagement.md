# Rules of engagement

These rules bind every run of this harness. The scope document records the
SHA-256 of this file; a run whose scope hash does not match the committed
file is refused.

1. Targets are limited to the hosts, ports and schemes listed in a signed
   scope document. Everything else is out of scope and the gate denies it.
2. Only intentionally vulnerable training applications and the owner's own
   assets are ever listed. No third-party system is ever a target.
3. Checks are non-destructive: no data deletion, no account lockout, no
   denial of service, no persistence beyond a marker in a field the checker
   itself created.
4. The harness contains no capability intended to avoid detection.
5. Rate limits in the scope document are enforced by the client, not
   requested of the target.
6. A finding is reported as validated only when a deterministic checker
   reproduced the behavior inside scope at least twice.
7. Model transcripts are not retained. Audit records hold digests and
   bounded metadata.
8. Any denial, refusal or budget stop is recorded and reported, never
   retried by rephrasing.
