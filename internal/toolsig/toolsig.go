// Package toolsig signs and verifies tool manifests (and other JSON
// documents) with Ed25519 over RFC 8785 canonical bytes.
//
// A valid signature proves provenance only: the manifest was produced by a
// key in the trust store and has not changed since. It never authorizes
// execution; that is the policy engine's job. Reason codes follow the
// sovereign-assist controller (same owner), which this package ports.
package toolsig

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/canon"
	"github.com/joeyvictorino/assay/internal/model"
)

// Reason codes returned by Verify and VerifyDocument.
const (
	ReasonUnsigned         = "UNSIGNED"
	ReasonSignatureInvalid = "SIGNATURE_INVALID"
	ReasonUnknownSigner    = "UNKNOWN_SIGNER"
	ReasonProvenanceValid  = "PROVENANCE_VALID"
)

// Document field names used by SignDocument / VerifyDocument.
const (
	FieldSignature = "signature"
	FieldSigner    = "signer"
	FieldSignedAt  = "signed_at"
)

// Sentinel errors. Verify wraps them so callers can errors.Is against the
// reason as well as compare the returned reason string.
var (
	ErrUnsigned         = errors.New("toolsig: " + ReasonUnsigned)
	ErrSignatureInvalid = errors.New("toolsig: " + ReasonSignatureInvalid)
	ErrUnknownSigner    = errors.New("toolsig: " + ReasonUnknownSigner)
	ErrEmptyTrustStore  = errors.New("toolsig: trust store has no keys")
)

// KeyID derives the short identifier for a public key: the first 16 hex
// characters of its SHA-256.
func KeyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:])[:16]
}

// GenerateKey creates a fresh Ed25519 key pair from crypto/rand.
func GenerateKey() (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		// crypto/rand.Reader is documented never to fail; the process cannot
		// continue safely without randomness.
		panic("toolsig: " + err.Error())
	}
	return pub, priv
}

// Sign returns a copy of m with Signer, SignedAt and Signature set. The
// signature covers the canonical JSON of the manifest with Signature cleared.
func Sign(m model.ToolManifest, priv ed25519.PrivateKey, now time.Time) (model.ToolManifest, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return model.ToolManifest{}, errors.New("toolsig: private key must be 64 bytes")
	}
	if strings.TrimSpace(m.Name) == "" {
		return model.ToolManifest{}, errors.New("toolsig: manifest name is required")
	}
	pub := priv.Public().(ed25519.PublicKey)
	m.Signer = KeyID(pub)
	m.SignedAt = now.UTC().Format(time.RFC3339)
	m.Signature = ""
	msg, err := canon.Bytes(m)
	if err != nil {
		return model.ToolManifest{}, fmt.Errorf("toolsig: canonicalize manifest: %w", err)
	}
	m.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, msg))
	return m, nil
}

// SignDocument signs an arbitrary JSON object the same way Sign signs a
// manifest: "signer" and "signed_at" are set, "signature" is cleared, the
// canonical bytes are signed and the base64 signature is stored under
// "signature". The input map is not modified.
func SignDocument(doc map[string]any, priv ed25519.PrivateKey, now time.Time) (map[string]any, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("toolsig: private key must be 64 bytes")
	}
	if doc == nil {
		return nil, errors.New("toolsig: document is nil")
	}
	pub := priv.Public().(ed25519.PublicKey)
	out := make(map[string]any, len(doc)+3)
	for k, v := range doc {
		out[k] = v
	}
	out[FieldSigner] = KeyID(pub)
	out[FieldSignedAt] = now.UTC().Format(time.RFC3339)
	delete(out, FieldSignature)
	msg, err := canon.Bytes(out)
	if err != nil {
		return nil, fmt.Errorf("toolsig: canonicalize document: %w", err)
	}
	out[FieldSignature] = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, msg))
	return out, nil
}

// TrustStore is the set of public keys whose signatures are accepted.
type TrustStore struct {
	keys   map[string]ed25519.PublicKey // key id -> key
	labels map[string]string            // key id -> file stem
}

// NewTrustStore builds a store from in-memory keys (label -> key). It errors
// when no keys are given.
func NewTrustStore(keys map[string]ed25519.PublicKey) (*TrustStore, error) {
	ts := &TrustStore{keys: map[string]ed25519.PublicKey{}, labels: map[string]string{}}
	for label, pub := range keys {
		if len(pub) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("toolsig: key %q is not 32 bytes", label)
		}
		id := KeyID(pub)
		ts.keys[id] = pub
		ts.labels[id] = label
	}
	if len(ts.keys) == 0 {
		return nil, ErrEmptyTrustStore
	}
	return ts, nil
}

// LoadTrustStore reads every *.pub file in dir. Each file holds one
// base64-encoded raw 32-byte Ed25519 public key; the file name without the
// extension is the key's label. An empty or missing directory is an error:
// a verifier with no trusted keys must fail closed.
func LoadTrustStore(dir string) (*TrustStore, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("toolsig: read trust dir: %w", err)
	}
	keys := map[string]ed25519.PublicKey{}
	for _, e := range entries {
		if !e.Type().IsRegular() || !strings.HasSuffix(e.Name(), ".pub") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("toolsig: read %s: %w", e.Name(), err)
		}
		pub, err := ParsePublicKey(string(raw))
		if err != nil {
			return nil, fmt.Errorf("toolsig: %s: %w", e.Name(), err)
		}
		keys[strings.TrimSuffix(e.Name(), ".pub")] = pub
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrEmptyTrustStore, dir)
	}
	return NewTrustStore(keys)
}

// KeyIDs lists the trusted key ids in sorted order.
func (ts *TrustStore) KeyIDs() []string {
	ids := make([]string, 0, len(ts.keys))
	for id := range ts.keys {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Label returns the label (file stem) a key id was loaded under.
func (ts *TrustStore) Label(keyID string) string { return ts.labels[keyID] }

// Verify checks a manifest's signature against the store. It implements
// model.ManifestVerifier. err is nil only when reason is PROVENANCE_VALID.
func (ts *TrustStore) Verify(m model.ToolManifest) (keyID, reason string, err error) {
	if ts == nil || len(ts.keys) == 0 {
		return "", ReasonUnknownSigner, ErrEmptyTrustStore
	}
	if m.Signature == "" {
		return "", ReasonUnsigned, ErrUnsigned
	}
	pub, ok := ts.keys[m.Signer]
	if !ok {
		return m.Signer, ReasonUnknownSigner, ErrUnknownSigner
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return m.Signer, ReasonSignatureInvalid, ErrSignatureInvalid
	}
	body := m
	body.Signature = ""
	msg, err := canon.Bytes(body)
	if err != nil {
		return m.Signer, ReasonSignatureInvalid, fmt.Errorf("%w: %v", ErrSignatureInvalid, err)
	}
	if !ed25519.Verify(pub, msg, sig) {
		return m.Signer, ReasonSignatureInvalid, ErrSignatureInvalid
	}
	return m.Signer, ReasonProvenanceValid, nil
}

// VerifyDocument checks a document produced by SignDocument.
func (ts *TrustStore) VerifyDocument(doc map[string]any) (keyID, reason string, err error) {
	if ts == nil || len(ts.keys) == 0 {
		return "", ReasonUnknownSigner, ErrEmptyTrustStore
	}
	if doc == nil {
		return "", ReasonUnsigned, ErrUnsigned
	}
	sigStr, _ := doc[FieldSignature].(string)
	if sigStr == "" {
		return "", ReasonUnsigned, ErrUnsigned
	}
	signer, _ := doc[FieldSigner].(string)
	pub, ok := ts.keys[signer]
	if !ok {
		return signer, ReasonUnknownSigner, ErrUnknownSigner
	}
	sig, err := base64.StdEncoding.DecodeString(sigStr)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return signer, ReasonSignatureInvalid, ErrSignatureInvalid
	}
	body := make(map[string]any, len(doc))
	for k, v := range doc {
		if k != FieldSignature {
			body[k] = v
		}
	}
	msg, err := canon.Bytes(body)
	if err != nil {
		return signer, ReasonSignatureInvalid, fmt.Errorf("%w: %v", ErrSignatureInvalid, err)
	}
	if !ed25519.Verify(pub, msg, sig) {
		return signer, ReasonSignatureInvalid, ErrSignatureInvalid
	}
	return signer, ReasonProvenanceValid, nil
}

// EncodePublicKey renders a public key in trust-store file format.
func EncodePublicKey(pub ed25519.PublicKey) string {
	return base64.StdEncoding.EncodeToString(pub)
}

// ParsePublicKey parses trust-store file content: base64 of 32 raw bytes,
// surrounding whitespace ignored.
func ParsePublicKey(s string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("public key is not base64: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key is %d bytes, want %d", len(raw), ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}

// EncodePrivateKey renders the 32-byte seed as base64 (the key-file format).
func EncodePrivateKey(priv ed25519.PrivateKey) string {
	return base64.StdEncoding.EncodeToString(priv.Seed())
}

// ParsePrivateKey accepts base64 of either the 32-byte seed or the full
// 64-byte private key.
func ParsePrivateKey(s string) (ed25519.PrivateKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("private key is not base64: %w", err)
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	}
	return nil, fmt.Errorf("private key is %d bytes, want %d or %d", len(raw), ed25519.SeedSize, ed25519.PrivateKeySize)
}
