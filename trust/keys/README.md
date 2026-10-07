# Trust store

This directory is the trust store for signed tool manifests. The harness
loads it with `toolsig.LoadTrustStore` and refuses to run when it is empty:
a verifier with no trusted keys fails closed.

## Format

- One file per trusted signer, named `<label>.pub`. The label is free text
  (`owner`, `ci`, `release-2026q4`) and appears in `assay tools verify`
  output next to the key id.
- File content: the raw 32-byte Ed25519 public key, base64 (standard
  alphabet, with padding), optionally followed by a newline. Nothing else.
- Files that do not end in `.pub` are ignored. A `.pub` file that does not
  decode to exactly 32 bytes makes loading fail.

The key id shown everywhere (manifest `signer` field, verify output) is the
first 16 hex characters of SHA-256 over the raw public key.

## Generating and using keys

```sh
assay tools keygen --out ~/.config/assay/keys   # writes assay.key (0600) and assay.pub
cp ~/.config/assay/keys/assay.pub trust/keys/owner.pub
assay tools sign   --manifests internal/tools/manifests --key ~/.config/assay/keys/assay.key
assay tools verify --manifests internal/tools/manifests --trust trust/keys
```

Private keys never live in this repository. `*.key` is in `.gitignore`.

## What a signature means

A valid signature proves provenance only: the manifest was produced by the
holder of a trusted key and has not changed since. It does not authorize
anything. Authorization is decided by the policy engine, which receives the
verification result as one input among several (see ADR 0003).

## Rotation

Add the new `<label>.pub`, re-sign the manifests, verify, then delete the old
file. Manifests signed by a removed key fail with `UNKNOWN_SIGNER`.
