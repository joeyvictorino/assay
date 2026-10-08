# Signed tool definitions and trust stores for MCP tool manifests

Author: Joey Victorino, https://github.com/joeyvictorino/assay
Status: draft for discussion. This is not an adopted MCP proposal and has no standing in the MCP project.
Date: 2026-10-07

## Problem

An MCP client shows a model, and usually a human reviewer, a set of tool definitions: name, description, input schema and any annotations. Two properties of the current arrangement are a problem for anyone who wants to reason about what a tool is allowed to do.

First, definitions are mutable between review and execution. A server can return one definition when a person approves it and a different one on the next `tools/list`. A changed description can add instructions the model will follow, and a changed schema or annotation can widen what a call does. The reviewer's approval no longer describes what runs.

Second, definitions are trusted because of where they came from. A client that connects to a server accepts whatever that server says about its own tools. There is no way for a publisher to say "this definition is mine" in a form that survives caching, mirroring, republishing or a compromised transport hop, and no way for a client to say "I only accept definitions from these publishers".

In my own harness, a policy engine reasons over each tool's declared risk tier, capabilities and allowed agents. If those fields can be edited after review, the engine reasons over false inputs. The same applies to any MCP client that applies policy to tool metadata.

## Prior art

I fetched the pages below. Where a page could not be fetched I say so.

- MCP SEP guidelines, https://modelcontextprotocol.io/community/sep-guidelines. Defines the proposal process used in the specification repository.
- MCP discussion #2913, "Signed tool manifests: additive extension for tool-poisoning / rug pull defense", https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2913. Proposes optional Ed25519 signing of tool manifests, scoped to descriptor integrity and explicitly not to runtime authorization. The page shows the thread as closed. It describes a layered model in which descriptor integrity sits beside execution authorization and result trust. No SEP came out of it.
- MCP discussion #2189, "Tool Bill of Materials (TBOM)", https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/2189. Proposes SHA-256 over RFC 8785 canonicalized tool definitions with publisher signatures. A second author describes an independent design (CTMS) with the same choices. Closed on 2026-09-25 with participants pointed to other channels.
- MCP discussion #348, "Client side tool validation to defend against tool poisoning", https://github.com/modelcontextprotocol/modelcontextprotocol/discussions/348. Proposes client-side baselines and drift detection. A maintainer judged it beyond protocol scope and suggested documenting best practice. Closed 2026-09-25.
- OWASP MCP Security Cheat Sheet, https://cheatsheetseries.owasp.org/cheatsheets/MCP_Security_Cheat_Sheet.html. Recommends pinning reviewed tool definitions with cryptographic hashes and re-review on change. It notes that pinning detects metadata changes, not behavior changes, and that optional signing drafts such as the MCPS Internet-Draft are individual work, not adopted MCP standards.
- ETDI, https://arxiv.org/abs/2506.01333. A paper proposing OAuth-enhanced, signed tool definitions with immutable versions. I read only the search result summary, not the paper, so I do not characterize it further.

The OWASP GenAI Security Project pages under genai.owasp.org returned HTTP 403 and were not read.

This proposal overlaps most with #2913 and #2189. The difference is narrow: a concrete trust store format, a fixed set of verification reason codes, and a working implementation with tests.

## Proposal

### Fields

A tool definition MAY carry three additional fields.

- `signer`: the key id of the signing key, defined below.
- `signed_at`: an RFC 3339 UTC timestamp.
- `signature`: base64 of the 64-byte Ed25519 signature.

### Signing

The signed message is the RFC 8785 canonical JSON of the tool definition with the `signature` field removed. `signer` and `signed_at` are inside the signed bytes. Any change to any other field, including fields a client does not currently read, invalidates the signature.

RFC 8785 specifies a number format that is hard to reproduce across languages. I suggest signed definitions MUST NOT contain non-integer numbers. This is a restriction on `inputSchema` contents (for example fractional `minimum` values) and it should be weighed against the benefit of byte-exact agreement between implementations. The reference implementation rejects such definitions rather than guessing.

### Trust store

A trust store is a set of Ed25519 public keys held by the client. The key id is the first 16 hex characters of the SHA-256 of the raw 32-byte public key. A client looks up `signer` in its store. The reference implementation stores one base64 key per file, with an operator-chosen label, and fails closed when the store is empty. The storage format is a client matter; only the key id derivation needs to be shared.

### Verification result

A verifier returns exactly one reason.

- `UNSIGNED`: no signature present.
- `UNKNOWN_SIGNER`: a signature is present but `signer` is not in the trust store.
- `SIGNATURE_INVALID`: the signer is known but the signature is malformed, the definition cannot be canonicalized, or verification fails.
- `PROVENANCE_VALID`: the signature verifies under a trusted key.

### What a signature means

A valid signature proves provenance and integrity only. It says a holder of a trusted key produced this exact definition. It does not say the tool is safe, that its implementation matches its description, or that a particular call is permitted. Authorization stays with the client's policy. In the reference harness the verification result is the first input to the policy engine, and every other check runs after it.

### Unsigned definitions

Clients choose by configuration how to treat anything other than `PROVENANCE_VALID`.

- `deny`: do not expose the tool to the model.
- `warn`: expose it, surface the reason to the user and record it.
- `allow`: expose it as today.

The default for existing clients would be `allow`, which preserves current behavior. High-assurance deployments would use `deny`.

## Compatibility

All three fields are optional. A server that does not sign sends the same bytes it sends today, and a client that does not verify ignores the extra fields. Because verification covers the whole definition, a client that strips or rewrites fields before verifying will see `SIGNATURE_INVALID`, so verification should run on the definition as received. Whether the fields belong in the core tool schema or in an extension is an open question, since the SEP guidelines describe an Extensions Track for optional additions.

## Reference implementation

The implementation is in Go, in the assay repository by the same author. It is a reference for the scheme, not an MCP SDK integration, and I make no claim of use by anyone other than myself.

- Signing, verification and trust store: https://github.com/joeyvictorino/assay/blob/main/internal/toolsig/toolsig.go
- Tests (sign then verify, tampering with each field, reason codes, trust store loading, key encoding): https://github.com/joeyvictorino/assay/blob/main/internal/toolsig/toolsig_test.go
- RFC 8785 canonicalization: https://github.com/joeyvictorino/assay/blob/main/internal/canon/canon.go
- Canonicalization tests, including the RFC 8785 vectors except the float array: https://github.com/joeyvictorino/assay/blob/main/internal/canon/canon_test.go
- Design records: https://github.com/joeyvictorino/assay/blob/main/docs/adr/0002-canonical-json-in-repo.md and https://github.com/joeyvictorino/assay/blob/main/docs/adr/0003-ed25519-signed-tool-manifests-and-trust-store.md
- A signed example manifest: https://github.com/joeyvictorino/assay/blob/main/internal/tools/manifests/http_get.json
- Trust store format and rotation notes: https://github.com/joeyvictorino/assay/blob/main/trust/keys/README.md

The manifest format in assay is its own (it includes fields such as `risk_tier` and `allowed_agents`), not the MCP tool schema. Mapping to MCP's `Tool` object is untested work.

## Open questions

1. Key distribution. How does a client learn which publisher keys to trust? Options include manual configuration, a registry, or something tied to server identity. This proposal does not answer it.
2. Rotation and revocation. The reference scheme rotates by adding a key, re-signing and removing the old key. Removal makes old definitions fail as `UNKNOWN_SIGNER`. There is no revocation protocol and no signature expiry beyond `signed_at`, which the verifier does not currently enforce.
3. Multiple signers. A definition might need a publisher signature and a reviewer or registry countersignature. A single `signature` field does not express that.
4. Dynamic tool lists. Servers may add, remove or parameterize tools at runtime. Signing a whole list, each tool, or a list with change notifications leads to different tradeoffs, as does replay of an older valid definition. A version or monotonic counter inside the signed bytes may be needed.
5. Relationship to MCP authorization. OAuth in MCP authorizes a client to a server. Signing authenticates a definition to a publisher. They answer different questions, and the interaction when the publisher and the server differ needs specification.
6. Scope. Discussion #348 shows a maintainer view that some of this belongs in documented client practice rather than the protocol. A guidance-only outcome, with the canonicalization and key id rules written down, may be the right result.

## Where to submit

Recommended: the MCP Contributor Discord, then a SEP only if there is interest.

The SEP guidelines (https://modelcontextprotocol.io/community/sep-guidelines) require prior discussion with the relevant working or interest group before a SEP pull request, with the discussion linked in the PR. They also require a sponsor from the maintainers list, a prototype, and note that the repository accepts pull requests from collaborators only. The communication page (https://modelcontextprotocol.io/community/communication) names `#security-ig` as an example of a group channel; confirm it exists once in the server.

Steps:
1. Join the Discord at https://discord.gg/6CSzBmMkjX.
2. Post a short summary in the security interest group channel, linking this document, discussions #2913 and #2189, and the reference code. Ask whether the authors of those threads and the group want a single combined effort, and whether the preferred outcome is an extension, a SEP or guidance text.
3. If there is interest, record the discussion link, copy this document into `seps/0000-signed-tool-definitions.md` using the SEP format (preamble, abstract, motivation, specification, rationale, backward compatibility, reference implementation, security implications), open a PR, rename the file to the PR number, and tag one or two maintainers from MAINTAINERS.md as sponsor.
4. Expect to be asked to merge with the existing signed-manifest work rather than compete with it.

Second choice: OWASP Cheat Sheet Series (https://github.com/OWASP/CheatSheetSeries). Open an issue or pull request against the MCP Security Cheat Sheet proposing a short section describing canonical-JSON signing and key id derivation as an option for the "pin reviewed definitions" guidance. The contribution route is the repository linked from the cheat sheet page; I did not read its CONTRIBUTING file, so check it first. The cheat sheet currently treats signing cautiously, so propose it as an optional technique with the provenance-only caveat.
