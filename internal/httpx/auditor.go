package httpx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"sync"

	"github.com/joeyvictorino/assay/internal/model"
)

// LEAD: these two auditors exist so S3 packages and the S3 CLI commands can
// run before internal/audit (S1) lands. They are plaintext and are not the
// production audit log. Replace with internal/audit when wiring the binary.

// MemAuditor keeps records in memory with a sha256 prev-hash chain (tests).
type MemAuditor struct {
	mu      sync.Mutex
	Records []model.AuditRecord
	head    string
	FailOn  func(rec model.AuditRecord) error // optional injected failure
}

// Record appends rec and returns its sequence number.
func (a *MemAuditor) Record(_ context.Context, rec model.AuditRecord) (uint64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.FailOn != nil {
		if err := a.FailOn(rec); err != nil {
			return 0, err
		}
	}
	rec.Seq = uint64(len(a.Records) + 1)
	rec.PrevHash = a.head
	b, _ := json.Marshal(rec)
	sum := sha256.Sum256(b)
	a.head = hex.EncodeToString(sum[:])
	a.Records = append(a.Records, rec)
	return rec.Seq, nil
}

// ByKind returns the records with the given Kind.
func (a *MemAuditor) ByKind(kind string) []model.AuditRecord {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []model.AuditRecord
	for _, r := range a.Records {
		if r.Kind == kind {
			out = append(out, r)
		}
	}
	return out
}

// Head returns the current chain head hash.
func (a *MemAuditor) Head() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.head
}

// WriterAuditor writes one JSON record per line to W with a prev-hash chain.
type WriterAuditor struct {
	W    io.Writer
	mu   sync.Mutex
	seq  uint64
	head string
}

// Record writes rec as a JSON line and returns its sequence number.
func (a *WriterAuditor) Record(_ context.Context, rec model.AuditRecord) (uint64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.seq++
	rec.Seq = a.seq
	rec.PrevHash = a.head
	b, err := json.Marshal(rec)
	if err != nil {
		return 0, err
	}
	sum := sha256.Sum256(b)
	a.head = hex.EncodeToString(sum[:])
	if _, err := a.W.Write(append(b, '\n')); err != nil {
		return 0, err
	}
	return rec.Seq, nil
}
