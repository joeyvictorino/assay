// Package audit is the append-only, hash-chained, AES-256-GCM encrypted
// record of every model call, tool call and decision in a run.
//
// File format (JSON lines):
//
//	{"v":1,"run_id":"...","key_id":"...","started":"..."}
//	{"seq":1,"prev":"<hex>","nonce":"<b64>","ct":"<b64>","hash":"<hex>"}
//	...
//
// hash = hex sha256(prev || nonce || ct) where prev is the previous line's
// hash (hex text) and the genesis prev is hex sha256("assay-genesis:"+run_id).
// ct is AES-256-GCM over the RFC 8785 canonical bytes of model.AuditRecord
// with additional data canon({"prev","run_id","seq"}). The per-run key is
// HKDF-SHA256(master, salt=run_id, info="assay-audit-v1").
//
// Records never carry bodies, prompts or secrets: the writer rejects any
// string value that is longer than 256 bytes or matches redact.ContainsSecret.
// Hash-chained receipts and the protected-content assertion are ported from
// the sovereign-assist receipt chain (same owner).
package audit

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joeyvictorino/assay/internal/canon"
	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/redact"
)

const (
	// Version is the file format version written in the header.
	Version = 1
	// Info is the HKDF info string that binds subkeys to this format.
	Info = "assay-audit-v1"
	// MasterKeySize is the required length of the master key.
	MasterKeySize = 32
	// MaxStringBytes bounds every string value inside a record.
	MaxStringBytes = 256

	nonceSize   = 12
	genesisTag  = "assay-genesis:"
	maxLineSize = 1 << 20
)

// Sentinel errors. Verification errors are wrapped in a *LineError that
// carries the 1-based line number; use errors.Is for the kind and errors.As
// for the line.
var (
	ErrChainBreak       = errors.New("audit: chain break")
	ErrHashMismatch     = errors.New("audit: hash mismatch")
	ErrWrongKey         = errors.New("audit: wrong key")
	ErrTruncated        = errors.New("audit: truncated log")
	ErrMalformed        = errors.New("audit: malformed line")
	ErrNonceReuse       = errors.New("audit: nonce reuse")
	ErrProtectedContent = errors.New("audit: record carries protected content")
	ErrRunMismatch      = errors.New("audit: record run_id does not match log")
	ErrClosed           = errors.New("audit: writer is closed")
)

// LineError locates a verification failure.
type LineError struct {
	Err    error
	Line   int
	Detail string
}

func (e *LineError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("%v (line %d)", e.Err, e.Line)
	}
	return fmt.Sprintf("%v (line %d): %s", e.Err, e.Line, e.Detail)
}

// Unwrap exposes the sentinel for errors.Is.
func (e *LineError) Unwrap() error { return e.Err }

func lineErr(err error, line int, detail string) error {
	return &LineError{Err: err, Line: line, Detail: detail}
}

type header struct {
	V       int    `json:"v"`
	RunID   string `json:"run_id"`
	KeyID   string `json:"key_id"`
	Started string `json:"started"`
}

type line struct {
	Seq   uint64 `json:"seq"`
	Prev  string `json:"prev"`
	Nonce string `json:"nonce"`
	CT    string `json:"ct"`
	Hash  string `json:"hash"`
}

// deriveKey returns the per-run AEAD and its key id.
func deriveKey(master []byte, runID string) (cipher.AEAD, string, error) {
	if len(master) != MasterKeySize {
		return nil, "", fmt.Errorf("audit: master key is %d bytes, want %d", len(master), MasterKeySize)
	}
	if runID == "" || strings.ContainsAny(runID, "\n\r") {
		return nil, "", errors.New("audit: run id is empty or contains line breaks")
	}
	sub, err := hkdf.Key(sha256.New, master, []byte(runID), Info, 32)
	if err != nil {
		return nil, "", fmt.Errorf("audit: hkdf: %w", err)
	}
	block, err := aes.NewCipher(sub)
	if err != nil {
		return nil, "", fmt.Errorf("audit: aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, "", fmt.Errorf("audit: gcm: %w", err)
	}
	sum := sha256.Sum256(sub)
	return aead, hex.EncodeToString(sum[:])[:16], nil
}

// GenesisHash is the prev value of the first record of a run.
func GenesisHash(runID string) string {
	sum := sha256.Sum256([]byte(genesisTag + runID))
	return hex.EncodeToString(sum[:])
}

func lineHash(prev string, nonce, ct []byte) string {
	h := sha256.New()
	h.Write([]byte(prev))
	h.Write(nonce)
	h.Write(ct)
	return hex.EncodeToString(h.Sum(nil))
}

func aadFor(seq uint64, prev, runID string) ([]byte, error) {
	return canon.Bytes(map[string]any{"seq": seq, "prev": prev, "run_id": runID})
}

// seal builds one encrypted line. It is separate from Record so tests can
// exercise the verifier with a chosen nonce.
func seal(aead cipher.AEAD, runID string, nonce []byte, rec model.AuditRecord) (line, error) {
	pt, err := canon.Bytes(rec)
	if err != nil {
		return line{}, fmt.Errorf("audit: canonicalize record: %w", err)
	}
	aad, err := aadFor(rec.Seq, rec.PrevHash, runID)
	if err != nil {
		return line{}, err
	}
	ct := aead.Seal(nil, nonce, pt, aad)
	return line{
		Seq:   rec.Seq,
		Prev:  rec.PrevHash,
		Nonce: base64.StdEncoding.EncodeToString(nonce),
		CT:    base64.StdEncoding.EncodeToString(ct),
		Hash:  lineHash(rec.PrevHash, nonce, ct),
	}, nil
}

// checkProtected enforces the bodies-free invariant on a record.
func checkProtected(rec model.AuditRecord) error {
	check := func(field, s string) error {
		if len(s) > MaxStringBytes {
			return fmt.Errorf("%w: %s is %d bytes (max %d)", ErrProtectedContent, field, len(s), MaxStringBytes)
		}
		if redact.ContainsSecret(s) {
			return fmt.Errorf("%w: %s matches a secret pattern", ErrProtectedContent, field)
		}
		return nil
	}
	for field, s := range map[string]string{
		"run_id": rec.RunID, "trace_id": rec.TraceID, "task_id": rec.TaskID, "agent": rec.Agent,
		"kind": rec.Kind, "tool_call_id": rec.ToolCallID, "tool": rec.Tool, "args_digest": rec.ArgsDigest,
	} {
		if err := check(field, s); err != nil {
			return err
		}
	}
	for k, v := range rec.Hashes {
		if err := check("hashes."+k, k); err != nil {
			return err
		}
		if err := check("hashes."+k, v); err != nil {
			return err
		}
	}
	return checkMeta("meta", rec.Meta, check, 0)
}

func checkMeta(path string, v any, check func(field, s string) error, depth int) error {
	if depth > 32 {
		return fmt.Errorf("%w: %s nests too deeply", ErrProtectedContent, path)
	}
	switch x := v.(type) {
	case string:
		return check(path, x)
	case map[string]any:
		for k, e := range x {
			if err := check(path+"."+k, k); err != nil {
				return err
			}
			if err := checkMeta(path+"."+k, e, check, depth+1); err != nil {
				return err
			}
		}
	case map[string]string:
		for k, e := range x {
			if err := check(path+"."+k, k); err != nil {
				return err
			}
			if err := check(path+"."+k, e); err != nil {
				return err
			}
		}
	case []any:
		for i, e := range x {
			if err := checkMeta(fmt.Sprintf("%s[%d]", path, i), e, check, depth+1); err != nil {
				return err
			}
		}
	case []string:
		for i, e := range x {
			if err := check(fmt.Sprintf("%s[%d]", path, i), e); err != nil {
				return err
			}
		}
	}
	return nil
}

// Writer appends records to one run's log. It is safe for concurrent use.
type Writer struct {
	mu     sync.Mutex
	f      *os.File
	runID  string
	keyID  string
	aead   cipher.AEAD
	seq    uint64
	head   string
	closed bool
}

// OpenWriter opens (or creates) the log at path for runID. master is the
// 32-byte master key; the per-run key is derived from it. When the file
// already holds records, the whole chain is verified with the derived key
// and the writer continues from its head; any inconsistency is an error.
func OpenWriter(path, runID string, master []byte) (*Writer, error) {
	aead, keyID, err := deriveKey(master, runID)
	if err != nil {
		return nil, err
	}
	w := &Writer{runID: runID, keyID: keyID, aead: aead, head: GenesisHash(runID)}

	st, err := os.Stat(path)
	switch {
	case err == nil && st.Size() > 0:
		sum, err := walk(path, master, nil)
		if err != nil {
			return nil, fmt.Errorf("audit: reopen %s: %w", path, err)
		}
		if sum.RunID != runID {
			return nil, fmt.Errorf("%w: log is for run %q, writer is for %q", ErrRunMismatch, sum.RunID, runID)
		}
		w.seq, w.head = sum.HeadSeq, sum.HeadHash
	case err == nil, errors.Is(err, os.ErrNotExist):
	default:
		return nil, fmt.Errorf("audit: stat %s: %w", path, err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("audit: open %s: %w", path, err)
	}
	w.f = f
	if w.seq == 0 && (st == nil || st.Size() == 0) {
		h := header{V: Version, RunID: runID, KeyID: keyID, Started: time.Now().UTC().Format(time.RFC3339Nano)}
		if err := w.writeLine(h); err != nil {
			f.Close()
			return nil, err
		}
	}
	return w, nil
}

func (w *Writer) writeLine(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("audit: encode line: %w", err)
	}
	b = append(b, '\n')
	n, err := w.f.Write(b)
	if err != nil {
		return fmt.Errorf("audit: write: %w", err)
	}
	if n != len(b) {
		return fmt.Errorf("audit: short write (%d of %d bytes)", n, len(b))
	}
	return nil
}

// KeyID is the identifier of the derived per-run key.
func (w *Writer) KeyID() string { return w.keyID }

// RunID is the run this writer logs.
func (w *Writer) RunID() string { return w.runID }

// Head returns the sequence number and hash of the last record (0 and the
// genesis hash for an empty log).
func (w *Writer) Head() (seq uint64, hash string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.seq, w.head
}

// Record implements model.Auditor. Seq and PrevHash are assigned by the
// writer; RunID is filled when empty and must otherwise match. Records with
// protected content are rejected before anything is written.
func (w *Writer) Record(ctx context.Context, rec model.AuditRecord) (uint64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if rec.RunID == "" {
		rec.RunID = w.runID
	} else if rec.RunID != w.runID {
		return 0, ErrRunMismatch
	}
	if strings.TrimSpace(rec.Kind) == "" {
		return 0, errors.New("audit: record kind is required")
	}
	if err := checkProtected(rec); err != nil {
		return 0, err
	}
	if rec.Time.IsZero() {
		rec.Time = time.Now().UTC()
	}
	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return 0, fmt.Errorf("audit: nonce: %w", err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, ErrClosed
	}
	rec.Seq = w.seq + 1
	rec.PrevHash = w.head
	l, err := seal(w.aead, w.runID, nonce, rec)
	if err != nil {
		return 0, err
	}
	if err := w.writeLine(l); err != nil {
		return 0, err
	}
	w.seq, w.head = l.Seq, l.Hash
	return w.seq, nil
}

// Close releases the file. Further Record calls fail with ErrClosed.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	return w.f.Close()
}

// Summary is the result of a successful verification.
type Summary struct {
	RunID    string         `json:"run_id"`
	KeyID    string         `json:"key_id"`
	Started  string         `json:"started"`
	Records  uint64         `json:"records"`
	HeadSeq  uint64         `json:"head_seq"`
	HeadHash string         `json:"head_hash"`
	Keyed    bool           `json:"keyed"`
	ByKind   map[string]int `json:"by_kind,omitempty"`
}

// Verify checks the log at path. With master nil only the header and the
// ciphertext hash chain are checked. With master set every record is also
// decrypted, its inner seq/prev/run_id compared with the line, the
// protected-content rule re-applied and records counted by Kind.
func Verify(path string, master []byte) (Summary, error) {
	return walk(path, master, nil)
}

// Export verifies the log with master and writes each decrypted record to w
// as one JSON line.
func Export(path string, master []byte, w io.Writer) (Summary, error) {
	if master == nil {
		return Summary{}, errors.New("audit: export requires the master key")
	}
	enc := json.NewEncoder(w)
	return walk(path, master, func(rec model.AuditRecord) error {
		return enc.Encode(rec)
	})
}

func walk(path string, master []byte, fn func(model.AuditRecord) error) (Summary, error) {
	f, err := os.Open(path)
	if err != nil {
		return Summary{}, fmt.Errorf("audit: open %s: %w", path, err)
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 64<<10)

	readLine := func(n int) ([]byte, error) {
		b, err := r.ReadBytes('\n')
		switch {
		case err == nil:
			if len(b) > maxLineSize {
				return nil, lineErr(ErrMalformed, n, "line too long")
			}
			return b[:len(b)-1], nil
		case errors.Is(err, io.EOF):
			if len(b) == 0 {
				return nil, io.EOF
			}
			return nil, lineErr(ErrTruncated, n, "last line has no newline")
		default:
			return nil, fmt.Errorf("audit: read: %w", err)
		}
	}

	hb, err := readLine(1)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return Summary{}, lineErr(ErrTruncated, 1, "missing header")
		}
		return Summary{}, err
	}
	var h header
	if err := strictUnmarshal(hb, &h); err != nil || h.V != Version || h.RunID == "" || len(h.KeyID) != 16 {
		return Summary{}, lineErr(ErrMalformed, 1, "bad header")
	}
	sum := Summary{RunID: h.RunID, KeyID: h.KeyID, Started: h.Started, HeadHash: GenesisHash(h.RunID)}

	var aead cipher.AEAD
	if master != nil {
		var keyID string
		aead, keyID, err = deriveKey(master, h.RunID)
		if err != nil {
			return Summary{}, err
		}
		if keyID != h.KeyID {
			return Summary{}, lineErr(ErrWrongKey, 1, "key id mismatch")
		}
		sum.Keyed = true
		sum.ByKind = map[string]int{}
	}

	nonces := map[string]struct{}{}
	for n := 2; ; n++ {
		lb, err := readLine(n)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Summary{}, err
		}
		var l line
		if err := strictUnmarshal(lb, &l); err != nil {
			return Summary{}, lineErr(ErrMalformed, n, err.Error())
		}
		nonce, err := base64.StdEncoding.DecodeString(l.Nonce)
		if err != nil || len(nonce) != nonceSize {
			return Summary{}, lineErr(ErrMalformed, n, "bad nonce")
		}
		ct, err := base64.StdEncoding.DecodeString(l.CT)
		if err != nil {
			return Summary{}, lineErr(ErrMalformed, n, "bad ciphertext encoding")
		}
		if lineHash(l.Prev, nonce, ct) != l.Hash {
			return Summary{}, lineErr(ErrHashMismatch, n, "line hash does not cover its content")
		}
		if l.Seq != sum.HeadSeq+1 {
			return Summary{}, lineErr(ErrChainBreak, n, fmt.Sprintf("seq %d after %d", l.Seq, sum.HeadSeq))
		}
		if l.Prev != sum.HeadHash {
			return Summary{}, lineErr(ErrChainBreak, n, "prev does not match previous hash")
		}
		if _, dup := nonces[l.Nonce]; dup {
			return Summary{}, lineErr(ErrNonceReuse, n, "nonce already used in this log")
		}
		nonces[l.Nonce] = struct{}{}

		if aead != nil {
			aad, err := aadFor(l.Seq, l.Prev, h.RunID)
			if err != nil {
				return Summary{}, err
			}
			pt, err := aead.Open(nil, nonce, ct, aad)
			if err != nil {
				return Summary{}, lineErr(ErrWrongKey, n, "decrypt failed")
			}
			var rec model.AuditRecord
			if err := strictUnmarshal(pt, &rec); err != nil {
				return Summary{}, lineErr(ErrMalformed, n, "plaintext is not an audit record")
			}
			if rec.Seq != l.Seq || rec.PrevHash != l.Prev || rec.RunID != h.RunID {
				return Summary{}, lineErr(ErrHashMismatch, n, "inner seq/prev/run_id differ from line")
			}
			if err := checkProtected(rec); err != nil {
				return Summary{}, lineErr(ErrProtectedContent, n, err.Error())
			}
			sum.ByKind[rec.Kind]++
			if fn != nil {
				if err := fn(rec); err != nil {
					return Summary{}, err
				}
			}
		}
		sum.HeadSeq, sum.HeadHash = l.Seq, l.Hash
		sum.Records++
	}
	return sum, nil
}

func strictUnmarshal(b []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data")
	}
	return nil
}

// GenerateMaster returns a fresh 32-byte master key.
func GenerateMaster() []byte {
	k := make([]byte, MasterKeySize)
	if _, err := rand.Read(k); err != nil {
		panic("audit: " + err.Error())
	}
	return k
}

// EncodeMaster renders a master key for an environment variable.
func EncodeMaster(k []byte) string { return base64.StdEncoding.EncodeToString(k) }

// ParseMaster accepts a base64 (standard or raw) or 64-hex master key.
func ParseMaster(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("audit: master key is empty")
	}
	var raw []byte
	var err error
	switch {
	case len(s) == 64 && isHex(s):
		raw, err = hex.DecodeString(s)
	case strings.HasSuffix(s, "="):
		raw, err = base64.StdEncoding.DecodeString(s)
	default:
		raw, err = base64.RawStdEncoding.DecodeString(s)
	}
	if err != nil {
		return nil, fmt.Errorf("audit: master key is not base64 or hex: %w", err)
	}
	if len(raw) != MasterKeySize {
		return nil, fmt.Errorf("audit: master key is %d bytes, want %d", len(raw), MasterKeySize)
	}
	return raw, nil
}

// MasterFromEnv reads and parses the master key from the named variable.
func MasterFromEnv(name string) ([]byte, error) {
	if name == "" {
		return nil, errors.New("audit: key environment variable name is empty")
	}
	v, ok := os.LookupEnv(name)
	if !ok || v == "" {
		return nil, fmt.Errorf("audit: environment variable %s is not set", name)
	}
	return ParseMaster(v)
}

func isHex(s string) bool {
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
