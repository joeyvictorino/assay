package toolsig

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

var fixedNow = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

func sampleManifest() model.ToolManifest {
	return model.ToolManifest{
		Name:        "http.get",
		Version:     "1.0.0",
		Description: "Fetch a URL within scope",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"url": map[string]any{"type": "string"}},
			"required":   []any{"url"},
		},
		RiskTier:      model.TierLow,
		Capabilities:  []string{"http.read"},
		SideEffects:   false,
		TimeoutMS:     5000,
		AllowedAgents: []string{"recon"},
	}
}

func storeWith(t *testing.T, keys ...ed25519.PublicKey) *TrustStore {
	t.Helper()
	m := map[string]ed25519.PublicKey{}
	for i, k := range keys {
		m[string(rune('a'+i))] = k
	}
	ts, err := NewTrustStore(m)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func TestSignThenVerify(t *testing.T) {
	pub, priv := GenerateKey()
	ts := storeWith(t, pub)
	signed, err := Sign(sampleManifest(), priv, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if signed.Signer != KeyID(pub) || signed.SignedAt != "2026-10-07T12:00:00Z" || signed.Signature == "" {
		t.Fatalf("sign did not fill fields: %+v", signed)
	}
	kid, reason, err := ts.Verify(signed)
	if err != nil || reason != ReasonProvenanceValid || kid != KeyID(pub) {
		t.Fatalf("verify: kid=%s reason=%s err=%v", kid, reason, err)
	}
	var _ model.ManifestVerifier = ts
	if len(KeyID(pub)) != 16 {
		t.Fatalf("key id length %d", len(KeyID(pub)))
	}
}

func TestTamperAnyFieldIsSignatureInvalid(t *testing.T) {
	pub, priv := GenerateKey()
	ts := storeWith(t, pub)
	signed, err := Sign(sampleManifest(), priv, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(m *model.ToolManifest)
	}{
		{"name", func(m *model.ToolManifest) { m.Name = "http.post" }},
		{"version", func(m *model.ToolManifest) { m.Version = "1.0.1" }},
		{"description", func(m *model.ToolManifest) { m.Description += "." }},
		{"input schema", func(m *model.ToolManifest) { m.InputSchema["additionalProperties"] = true }},
		{"risk tier escalated down", func(m *model.ToolManifest) { m.RiskTier = model.TierCritical }},
		{"capability added", func(m *model.ToolManifest) { m.Capabilities = append(m.Capabilities, "http.write") }},
		{"capability reordered", func(m *model.ToolManifest) {
			m.Capabilities = []string{"http.read", "delegate"}
			m.Capabilities = m.Capabilities[:1]
			m.Capabilities = append([]string{"delegate"}, m.Capabilities...)
		}},
		{"side effects", func(m *model.ToolManifest) { m.SideEffects = true }},
		{"timeout", func(m *model.ToolManifest) { m.TimeoutMS = 60000 }},
		{"allowed agents", func(m *model.ToolManifest) { m.AllowedAgents = nil }},
		{"signed_at", func(m *model.ToolManifest) { m.SignedAt = "2027-01-01T00:00:00Z" }},
		{"signature bit flip", func(m *model.ToolManifest) {
			raw, _ := base64.StdEncoding.DecodeString(m.Signature)
			raw[0] ^= 0x01
			m.Signature = base64.StdEncoding.EncodeToString(raw)
		}},
		{"signature not base64", func(m *model.ToolManifest) { m.Signature = "%%%" }},
		{"signature wrong length", func(m *model.ToolManifest) { m.Signature = base64.StdEncoding.EncodeToString([]byte("short")) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := signed
			m.InputSchema = map[string]any{}
			for k, v := range signed.InputSchema {
				m.InputSchema[k] = v
			}
			m.Capabilities = append([]string(nil), signed.Capabilities...)
			tc.mutate(&m)
			_, reason, err := ts.Verify(m)
			if reason != ReasonSignatureInvalid || !errors.Is(err, ErrSignatureInvalid) {
				t.Fatalf("reason=%s err=%v", reason, err)
			}
		})
	}
}

func TestVerifyReasonCodes(t *testing.T) {
	pub, priv := GenerateKey()
	otherPub, otherPriv := GenerateKey()
	ts := storeWith(t, pub)

	unsigned := sampleManifest()
	if _, reason, err := ts.Verify(unsigned); reason != ReasonUnsigned || !errors.Is(err, ErrUnsigned) {
		t.Fatalf("unsigned: %s %v", reason, err)
	}

	signed, _ := Sign(sampleManifest(), priv, fixedNow)
	emptied := signed
	emptied.Signature = ""
	if _, reason, err := ts.Verify(emptied); reason != ReasonUnsigned || !errors.Is(err, ErrUnsigned) {
		t.Fatalf("emptied: %s %v", reason, err)
	}

	foreign, _ := Sign(sampleManifest(), otherPriv, fixedNow)
	if kid, reason, err := ts.Verify(foreign); reason != ReasonUnknownSigner || !errors.Is(err, ErrUnknownSigner) || kid != KeyID(otherPub) {
		t.Fatalf("foreign: %s %s %v", kid, reason, err)
	}

	// Claiming a trusted signer id over a foreign signature is still invalid.
	spoofed := foreign
	spoofed.Signer = KeyID(pub)
	if _, reason, _ := ts.Verify(spoofed); reason != ReasonSignatureInvalid {
		t.Fatalf("spoofed signer: %s", reason)
	}

	// A store with both keys accepts both.
	both := storeWith(t, pub, otherPub)
	if _, reason, err := both.Verify(foreign); reason != ReasonProvenanceValid || err != nil {
		t.Fatalf("both: %s %v", reason, err)
	}
	if got := both.KeyIDs(); len(got) != 2 {
		t.Fatalf("key ids: %v", got)
	}

	var nilStore *TrustStore
	if _, _, err := nilStore.Verify(signed); !errors.Is(err, ErrEmptyTrustStore) {
		t.Fatalf("nil store: %v", err)
	}
}

func TestSignRejectsBadInput(t *testing.T) {
	_, priv := GenerateKey()
	if _, err := Sign(model.ToolManifest{}, priv, fixedNow); err == nil {
		t.Fatal("expected error for nameless manifest")
	}
	if _, err := Sign(sampleManifest(), ed25519.PrivateKey("short"), fixedNow); err == nil {
		t.Fatal("expected error for short key")
	}
	m := sampleManifest()
	m.InputSchema["ratio"] = 0.5
	if _, err := Sign(m, priv, fixedNow); err == nil {
		t.Fatal("expected error for float in schema")
	}
}

func TestLoadTrustStore(t *testing.T) {
	pub, _ := GenerateKey()
	pub2, _ := GenerateKey()

	t.Run("empty dir fails closed", func(t *testing.T) {
		if _, err := LoadTrustStore(t.TempDir()); !errors.Is(err, ErrEmptyTrustStore) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("missing dir fails", func(t *testing.T) {
		if _, err := LoadTrustStore(filepath.Join(t.TempDir(), "nope")); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("ignores non-pub files, labels by stem", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, "owner.pub"), EncodePublicKey(pub)+"\n")
		write(t, filepath.Join(dir, "ci.pub"), "  "+EncodePublicKey(pub2))
		write(t, filepath.Join(dir, "README.md"), "not a key")
		if err := os.Mkdir(filepath.Join(dir, "sub.pub"), 0o755); err != nil {
			t.Fatal(err)
		}
		ts, err := LoadTrustStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		if ts.Label(KeyID(pub)) != "owner" || ts.Label(KeyID(pub2)) != "ci" {
			t.Fatalf("labels: %v %v", ts.Label(KeyID(pub)), ts.Label(KeyID(pub2)))
		}
	})
	t.Run("malformed key file fails", func(t *testing.T) {
		for name, content := range map[string]string{
			"not base64": "!!!",
			"wrong size": base64.StdEncoding.EncodeToString([]byte("too short")),
		} {
			dir := t.TempDir()
			write(t, filepath.Join(dir, "bad.pub"), content)
			if _, err := LoadTrustStore(dir); err == nil {
				t.Fatalf("%s: expected error", name)
			}
		}
	})
}

func TestSignVerifyDocument(t *testing.T) {
	pub, priv := GenerateKey()
	_, otherPriv := GenerateKey()
	ts := storeWith(t, pub)
	doc := map[string]any{
		"scope_id": "lab-01",
		"targets":  []any{"https://lab.internal"},
		"expires":  "2026-12-31T00:00:00Z",
		"nested":   map[string]any{"b": 1, "a": true},
	}
	signed, err := SignDocument(doc, priv, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := doc[FieldSignature]; ok {
		t.Fatal("input document was modified")
	}
	if signed[FieldSigner] != KeyID(pub) || signed[FieldSignedAt] != "2026-10-07T12:00:00Z" {
		t.Fatalf("fields: %v", signed)
	}
	if kid, reason, err := ts.VerifyDocument(signed); err != nil || reason != ReasonProvenanceValid || kid != KeyID(pub) {
		t.Fatalf("verify: %s %s %v", kid, reason, err)
	}

	cases := []struct {
		name   string
		mutate func(d map[string]any)
		reason string
	}{
		{"tamper target", func(d map[string]any) { d["targets"] = []any{"https://evil.example"} }, ReasonSignatureInvalid},
		{"tamper nested", func(d map[string]any) { d["nested"] = map[string]any{"b": 2, "a": true} }, ReasonSignatureInvalid},
		{"add field", func(d map[string]any) { d["extra"] = 1 }, ReasonSignatureInvalid},
		{"remove field", func(d map[string]any) { delete(d, "expires") }, ReasonSignatureInvalid},
		{"empty signature", func(d map[string]any) { d[FieldSignature] = "" }, ReasonUnsigned},
		{"missing signature", func(d map[string]any) { delete(d, FieldSignature) }, ReasonUnsigned},
		{"non-string signature", func(d map[string]any) { d[FieldSignature] = 42 }, ReasonUnsigned},
		{"unknown signer", func(d map[string]any) { d[FieldSigner] = "deadbeefdeadbeef" }, ReasonUnknownSigner},
		{"missing signer", func(d map[string]any) { delete(d, FieldSigner) }, ReasonUnknownSigner},
		{"garbage signature", func(d map[string]any) { d[FieldSignature] = "@@" }, ReasonSignatureInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := make(map[string]any, len(signed))
			for k, v := range signed {
				d[k] = v
			}
			tc.mutate(d)
			if _, reason, err := ts.VerifyDocument(d); reason != tc.reason || err == nil {
				t.Fatalf("reason=%s err=%v want %s", reason, err, tc.reason)
			}
		})
	}

	foreign, _ := SignDocument(doc, otherPriv, fixedNow)
	if _, reason, _ := ts.VerifyDocument(foreign); reason != ReasonUnknownSigner {
		t.Fatalf("foreign: %s", reason)
	}
	if _, reason, _ := ts.VerifyDocument(nil); reason != ReasonUnsigned {
		t.Fatalf("nil doc: %s", reason)
	}
	if _, err := SignDocument(nil, priv, fixedNow); err == nil {
		t.Fatal("nil doc should not sign")
	}
	if _, err := SignDocument(map[string]any{"f": 1.5}, priv, fixedNow); err == nil {
		t.Fatal("float doc should not sign")
	}
}

func TestKeyEncoding(t *testing.T) {
	pub, priv := GenerateKey()
	p2, err := ParsePublicKey(EncodePublicKey(pub) + "\n")
	if err != nil || !pub.Equal(p2) {
		t.Fatalf("public round trip: %v", err)
	}
	k2, err := ParsePrivateKey(EncodePrivateKey(priv))
	if err != nil || !priv.Equal(k2) {
		t.Fatalf("seed round trip: %v", err)
	}
	k3, err := ParsePrivateKey(base64.StdEncoding.EncodeToString(priv))
	if err != nil || !priv.Equal(k3) {
		t.Fatalf("full key round trip: %v", err)
	}
	for _, bad := range []string{"", "!!", base64.StdEncoding.EncodeToString([]byte("x"))} {
		if _, err := ParsePrivateKey(bad); err == nil {
			t.Fatalf("ParsePrivateKey(%q) should fail", bad)
		}
		if _, err := ParsePublicKey(bad); err == nil {
			t.Fatalf("ParsePublicKey(%q) should fail", bad)
		}
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
