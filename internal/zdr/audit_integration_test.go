package zdr

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/joeyvictorino/assay/internal/audit"
	"github.com/joeyvictorino/assay/internal/model"
)

// TestAuditRejectsPlaintextSecrets is the end-to-end zero-retention check for
// the audit plane: a record that smuggles a secret or a transcript canary is
// refused, and whatever did get written never contains either, even as
// ciphertext bytes.
func TestAuditRejectsPlaintextSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	master := audit.GenerateMaster()
	w, err := audit.OpenWriter(path, "run-zdr", master)
	if err != nil {
		t.Fatal(err)
	}
	canary := NewCanary()
	secret := "sk-ant-api03-" + canary[len(CanaryPrefix):] + "ABCDEFGH"

	// A legitimate record: digests and bounded metadata only.
	if _, err := w.Record(context.Background(), model.AuditRecord{
		Kind: "model_call", Agent: "recon",
		Hashes: map[string]string{"prompt": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		Meta:   map[string]any{"tokens": 10},
	}); err != nil {
		t.Fatal(err)
	}

	// A record carrying a secret is refused.
	if _, err := w.Record(context.Background(), model.AuditRecord{
		Kind: "model_call", Meta: map[string]any{"prompt": "please use " + secret},
	}); !errors.Is(err, audit.ErrProtectedContent) {
		t.Fatalf("secret accepted: %v", err)
	}

	// A record that pastes a long prompt (which contains the canary) is
	// refused by the length bound even though the canary is not a secret.
	long := "transcript: " + canary
	for len(long) <= audit.MaxStringBytes {
		long += " lorem ipsum"
	}
	if _, err := w.Record(context.Background(), model.AuditRecord{
		Kind: "model_call", Meta: map[string]any{"prompt": long},
	}); !errors.Is(err, audit.ErrProtectedContent) {
		t.Fatalf("long body accepted: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	if sum, err := audit.Verify(path, master); err != nil || sum.Records != 1 {
		t.Fatalf("verify: %+v %v", sum, err)
	}
	hits, err := ScanTree(dir, []string{canary, secret, "please use", "transcript:"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("protected content reached disk: %+v", hits)
	}
}
