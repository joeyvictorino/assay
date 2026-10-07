package audit

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

const testRun = "run-2026-10-07-0001"

var kinds = []string{"model_call", "tool_call", "policy_decision", "tool_call", "finding"}

func sampleRecord(i int) model.AuditRecord {
	return model.AuditRecord{
		Time:       time.Date(2026, 10, 7, 12, 0, i, 0, time.UTC),
		TraceID:    "trace-1",
		TaskID:     "task-1",
		Agent:      "recon",
		Kind:       kinds[i%len(kinds)],
		ToolCallID: "call-" + string(rune('a'+i)),
		Tool:       "http.get",
		ArgsDigest: strings.Repeat("ab", 32),
		Hashes:     map[string]string{"prompt": strings.Repeat("cd", 32), "response": strings.Repeat("ef", 32)},
		Meta:       map[string]any{"model": "m-1", "tokens": 1234, "latency_ms": 87, "cost_microusd": 4200, "decision": map[string]any{"effect": "allow", "reason": "ALLOW_RULE:x"}},
	}
}

// newLog writes n records and closes the writer.
func newLog(t *testing.T, n int) (path string, master []byte, head string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "audit.log")
	master = GenerateMaster()
	w, err := OpenWriter(path, testRun, master)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		seq, err := w.Record(context.Background(), sampleRecord(i))
		if err != nil {
			t.Fatal(err)
		}
		if seq != uint64(i+1) {
			t.Fatalf("seq %d want %d", seq, i+1)
		}
	}
	_, head = w.Head()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return path, master, head
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.TrimSuffix(string(b), "\n")
	return strings.Split(s, "\n")
}

func writeLines(t *testing.T, path string, lines []string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func wantLineErr(t *testing.T, err, kind error, line int) {
	t.Helper()
	if !errors.Is(err, kind) {
		t.Fatalf("err = %v, want %v", err, kind)
	}
	var le *LineError
	if !errors.As(err, &le) {
		t.Fatalf("err %v is not a LineError", err)
	}
	if le.Line != line {
		t.Fatalf("line %d want %d (%v)", le.Line, line, err)
	}
}

func TestRoundTrip(t *testing.T) {
	path, master, head := newLog(t, 5)
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode().Perm())
	}

	keyless, err := Verify(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if keyless.Records != 5 || keyless.HeadSeq != 5 || keyless.HeadHash != head || keyless.Keyed || keyless.RunID != testRun {
		t.Fatalf("keyless %+v", keyless)
	}

	keyed, err := Verify(path, master)
	if err != nil {
		t.Fatal(err)
	}
	if !keyed.Keyed || keyed.Records != 5 || keyed.HeadHash != head {
		t.Fatalf("keyed %+v", keyed)
	}
	if keyed.ByKind["tool_call"] != 2 || keyed.ByKind["model_call"] != 1 || keyed.ByKind["policy_decision"] != 1 || keyed.ByKind["finding"] != 1 {
		t.Fatalf("by kind %v", keyed.ByKind)
	}

	var out bytes.Buffer
	if _, err := Export(path, master, &out); err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(&out)
	prev := GenesisHash(testRun)
	n := 0
	for sc.Scan() {
		var rec model.AuditRecord
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			t.Fatal(err)
		}
		n++
		if rec.Seq != uint64(n) || rec.PrevHash != prev || rec.RunID != testRun {
			t.Fatalf("exported record %d: %+v", n, rec)
		}
		want := sampleRecord(n - 1)
		if rec.Kind != want.Kind || rec.Hashes["prompt"] != want.Hashes["prompt"] || rec.Meta["model"] != "m-1" || !rec.Time.Equal(want.Time) {
			t.Fatalf("exported record %d content: %+v", n, rec)
		}
		// prev for the next record is the line hash, read from the file.
		var l line
		if err := json.Unmarshal([]byte(readLines(t, path)[n]), &l); err != nil {
			t.Fatal(err)
		}
		prev = l.Hash
	}
	if n != 5 {
		t.Fatalf("exported %d records", n)
	}

	// The file never contains plaintext field values.
	raw, _ := os.ReadFile(path)
	for _, leak := range []string{"http.get", "recon", "trace-1", "ALLOW_RULE", strings.Repeat("cd", 32)} {
		if bytes.Contains(raw, []byte(leak)) {
			t.Fatalf("plaintext %q found in log", leak)
		}
	}
}

func TestHeaderOnlyLog(t *testing.T) {
	path, master, head := newLog(t, 0)
	if head != GenesisHash(testRun) {
		t.Fatalf("head %s", head)
	}
	lines := readLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("lines %v", lines)
	}
	var h header
	if err := json.Unmarshal([]byte(lines[0]), &h); err != nil || h.V != 1 || h.RunID != testRun || len(h.KeyID) != 16 || h.Started == "" {
		t.Fatalf("header %+v %v", h, err)
	}
	sum, err := Verify(path, master)
	if err != nil || sum.Records != 0 || sum.HeadHash != head {
		t.Fatalf("%+v %v", sum, err)
	}
}

func TestTamperOneByte(t *testing.T) {
	path, master, _ := newLog(t, 5)
	lines := readLines(t, path)
	var l line
	if err := json.Unmarshal([]byte(lines[3]), &l); err != nil {
		t.Fatal(err)
	}
	ct, _ := base64.StdEncoding.DecodeString(l.CT)
	ct[len(ct)/2] ^= 0x80
	l.CT = base64.StdEncoding.EncodeToString(ct)
	b, _ := json.Marshal(l)
	lines[3] = string(b)
	writeLines(t, path, lines)

	_, err := Verify(path, nil)
	wantLineErr(t, err, ErrHashMismatch, 4)
	_, err = Verify(path, master)
	wantLineErr(t, err, ErrHashMismatch, 4)

	// Tampering the hash field itself is also a hash mismatch on that line.
	lines = readLines(t, newLogPath(t, 3))
	lines[2] = strings.Replace(lines[2], `"hash":"`+hashOf(t, lines[2])[:4], `"hash":"`+flipHex(hashOf(t, lines[2])[:4]), 1)
	p := filepath.Join(t.TempDir(), "h.log")
	writeLines(t, p, lines)
	_, err = Verify(p, nil)
	wantLineErr(t, err, ErrHashMismatch, 3)
}

func newLogPath(t *testing.T, n int) string {
	p, _, _ := newLog(t, n)
	return p
}

func hashOf(t *testing.T, s string) string {
	var l line
	if err := json.Unmarshal([]byte(s), &l); err != nil {
		t.Fatal(err)
	}
	return l.Hash
}

func flipHex(s string) string {
	b := []byte(s)
	if b[0] == '0' {
		b[0] = '1'
	} else {
		b[0] = '0'
	}
	return string(b)
}

func TestDeleteMiddleLine(t *testing.T) {
	path, master, _ := newLog(t, 5)
	lines := readLines(t, path)
	lines = append(lines[:2], lines[3:]...) // drop record seq 2 (file line 3)
	writeLines(t, path, lines)
	_, err := Verify(path, nil)
	wantLineErr(t, err, ErrChainBreak, 3)
	_, err = Verify(path, master)
	wantLineErr(t, err, ErrChainBreak, 3)
}

func TestReorderedLinesAreChainBreak(t *testing.T) {
	path, _, _ := newLog(t, 4)
	lines := readLines(t, path)
	lines[2], lines[3] = lines[3], lines[2]
	writeLines(t, path, lines)
	_, err := Verify(path, nil)
	wantLineErr(t, err, ErrChainBreak, 3)
}

func TestTruncateLastLine(t *testing.T) {
	path, master, _ := newLog(t, 5)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b[:len(b)-17], 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Verify(path, nil)
	wantLineErr(t, err, ErrTruncated, 6)
	_, err = Verify(path, master)
	wantLineErr(t, err, ErrTruncated, 6)

	// A writer refuses to continue a truncated log.
	if _, err := OpenWriter(path, testRun, master); !errors.Is(err, ErrTruncated) {
		t.Fatalf("reopen: %v", err)
	}

	// An empty file is a missing header.
	empty := filepath.Join(t.TempDir(), "empty.log")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Verify(empty, nil)
	wantLineErr(t, err, ErrTruncated, 1)
}

func TestWrongKey(t *testing.T) {
	path, master, _ := newLog(t, 3)
	other := GenerateMaster()
	_, err := Verify(path, other)
	wantLineErr(t, err, ErrWrongKey, 1)
	if _, err := OpenWriter(path, testRun, other); !errors.Is(err, ErrWrongKey) {
		t.Fatalf("reopen with wrong key: %v", err)
	}
	// Same master, different run id: different key, different header.
	if _, err := OpenWriter(path, "other-run", master); !errors.Is(err, ErrWrongKey) && !errors.Is(err, ErrRunMismatch) {
		t.Fatalf("reopen with other run: %v", err)
	}
	// Keyless verification does not need the key at all.
	if _, err := Verify(path, nil); err != nil {
		t.Fatal(err)
	}
	// Export without a key is refused.
	if _, err := Export(path, nil, &bytes.Buffer{}); err == nil {
		t.Fatal("export without key should fail")
	}
}

func TestRecordRejectsProtectedContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	master := GenerateMaster()
	w, err := OpenWriter(path, testRun, master)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	base := sampleRecord(0)
	long := strings.Repeat("x", MaxStringBytes+1)
	cases := []struct {
		name   string
		mutate func(r *model.AuditRecord)
	}{
		{"api key in meta", func(r *model.AuditRecord) { r.Meta["note"] = "key sk-ant-api03-ABCDEFGH12345678abcdefgh" }},
		{"bearer in nested meta", func(r *model.AuditRecord) {
			r.Meta["decision"] = map[string]any{"hdr": "Authorization: Bearer abcdef123456"}
		}},
		{"jwt in meta slice", func(r *model.AuditRecord) {
			r.Meta["list"] = []any{"ok", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2lnbmF0dXJl"}
		}},
		{"secret in meta key", func(r *model.AuditRecord) { r.Meta["password=hunter2xyz"] = 1 }},
		{"long meta string (body smuggling)", func(r *model.AuditRecord) { r.Meta["body"] = long }},
		{"long hash value", func(r *model.AuditRecord) { r.Hashes["prompt"] = long }},
		{"secret in hashes", func(r *model.AuditRecord) { r.Hashes["x"] = "AKIAIOSFODNN7EXAMPLE" }},
		{"secret in tool name", func(r *model.AuditRecord) { r.Tool = "ghp_0123456789ABCDEFGHIJ0123456789ab" }},
		{"secret in args digest", func(r *model.AuditRecord) { r.ArgsDigest = "https://u:p4ssw0rd@h/" }},
		{"pem in meta", func(r *model.AuditRecord) {
			r.Meta["k"] = "-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----"
		}},
		{"long agent", func(r *model.AuditRecord) { r.Agent = long }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := sampleRecord(0)
			r.Meta = map[string]any{}
			for k, v := range base.Meta {
				r.Meta[k] = v
			}
			r.Hashes = map[string]string{}
			for k, v := range base.Hashes {
				r.Hashes[k] = v
			}
			tc.mutate(&r)
			if _, err := w.Record(context.Background(), r); !errors.Is(err, ErrProtectedContent) {
				t.Fatalf("err = %v, want ErrProtectedContent", err)
			}
		})
	}
	if seq, _ := w.Head(); seq != 0 {
		t.Fatalf("rejected records advanced the chain to %d", seq)
	}
	if lines := readLines(t, path); len(lines) != 1 {
		t.Fatalf("rejected records were written: %d lines", len(lines))
	}

	// Other rejections.
	if _, err := w.Record(context.Background(), model.AuditRecord{Kind: "x", RunID: "other"}); !errors.Is(err, ErrRunMismatch) {
		t.Fatalf("run mismatch: %v", err)
	}
	if _, err := w.Record(context.Background(), model.AuditRecord{}); err == nil {
		t.Fatal("kind required")
	}
	if _, err := w.Record(context.Background(), model.AuditRecord{Kind: "x", Meta: map[string]any{"ratio": 0.5}}); err == nil {
		t.Fatal("float meta should be rejected by canonicalization")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := w.Record(ctx, sampleRecord(0)); !errors.Is(err, context.Canceled) {
		t.Fatalf("ctx: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Record(context.Background(), sampleRecord(0)); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed: %v", err)
	}
}

func TestVerifyDetectsProtectedContentWrittenByOtherMeans(t *testing.T) {
	// Build a line directly with seal (bypassing Record's check) and make
	// sure keyed verification still catches it.
	path, master, head := newLog(t, 1)
	aead, _, err := deriveKey(master, testRun)
	if err != nil {
		t.Fatal(err)
	}
	rec := sampleRecord(1)
	rec.RunID, rec.Seq, rec.PrevHash = testRun, 2, head
	rec.Meta["leak"] = "sk-ABCDEFGH12345678"
	l, err := seal(aead, testRun, bytes.Repeat([]byte{7}, nonceSize), rec)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(l)
	lines := append(readLines(t, path), string(b))
	writeLines(t, path, lines)
	if _, err := Verify(path, nil); err != nil {
		t.Fatalf("keyless cannot see inside: %v", err)
	}
	_, err = Verify(path, master)
	wantLineErr(t, err, ErrProtectedContent, 3)
}

func TestReopenAppendContinuesChain(t *testing.T) {
	path, master, head := newLog(t, 3)
	w, err := OpenWriter(path, testRun, master)
	if err != nil {
		t.Fatal(err)
	}
	if seq, h := w.Head(); seq != 3 || h != head {
		t.Fatalf("head after reopen %d %s", seq, h)
	}
	for i := 3; i < 5; i++ {
		if _, err := w.Record(context.Background(), sampleRecord(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	sum, err := Verify(path, master)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Records != 5 || sum.HeadSeq != 5 {
		t.Fatalf("%+v", sum)
	}
	if lines := readLines(t, path); len(lines) != 6 || strings.Count(strings.Join(lines, "\n"), `"v":1`) != 1 {
		t.Fatalf("header duplicated or lines wrong: %d", len(lines))
	}
}

func TestReopenRefusesBrokenChain(t *testing.T) {
	path, master, _ := newLog(t, 3)
	lines := readLines(t, path)
	lines = append(lines[:1], lines[2:]...)
	writeLines(t, path, lines)
	if _, err := OpenWriter(path, testRun, master); !errors.Is(err, ErrChainBreak) {
		t.Fatalf("reopen: %v", err)
	}
}

func TestNonceReuseDetected(t *testing.T) {
	path, master, _ := newLog(t, 1)
	lines := readLines(t, path)
	var first line
	if err := json.Unmarshal([]byte(lines[1]), &first); err != nil {
		t.Fatal(err)
	}
	nonce, _ := base64.StdEncoding.DecodeString(first.Nonce)
	aead, _, err := deriveKey(master, testRun)
	if err != nil {
		t.Fatal(err)
	}
	rec := sampleRecord(1)
	rec.RunID, rec.Seq, rec.PrevHash = testRun, 2, first.Hash
	second, err := seal(aead, testRun, nonce, rec)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(second)
	writeLines(t, path, append(lines, string(b)))
	_, err = Verify(path, nil)
	wantLineErr(t, err, ErrNonceReuse, 3)
	_, err = Verify(path, master)
	wantLineErr(t, err, ErrNonceReuse, 3)
}

func TestInnerOuterMismatch(t *testing.T) {
	// A record whose encrypted seq disagrees with the line seq while the
	// outer chain is consistent: only keyed verification can see it.
	path, master, head := newLog(t, 1)
	aead, _, err := deriveKey(master, testRun)
	if err != nil {
		t.Fatal(err)
	}
	rec := sampleRecord(1)
	rec.RunID, rec.Seq, rec.PrevHash = testRun, 7, head
	l, err := seal(aead, testRun, bytes.Repeat([]byte{9}, nonceSize), rec)
	if err != nil {
		t.Fatal(err)
	}
	l.Seq = 2
	// AAD covers seq, so decryption fails rather than yielding a mismatch.
	b, _ := json.Marshal(l)
	writeLines(t, path, append(readLines(t, path), string(b)))
	if _, err := Verify(path, nil); err != nil {
		t.Fatal(err)
	}
	_, err = Verify(path, master)
	wantLineErr(t, err, ErrWrongKey, 3)
}

func TestMalformedLines(t *testing.T) {
	path, _, _ := newLog(t, 2)
	good := readLines(t, path)
	cases := []struct {
		name  string
		lines []string
		kind  error
		line  int
	}{
		{"garbage header", append([]string{"nope"}, good[1:]...), ErrMalformed, 1},
		{"wrong version", append([]string{strings.Replace(good[0], `"v":1`, `"v":2`, 1)}, good[1:]...), ErrMalformed, 1},
		{"garbage record", []string{good[0], "{not json"}, ErrMalformed, 2},
		{"unknown field", []string{good[0], strings.Replace(good[1], `"seq":1`, `"seq":1,"x":1`, 1)}, ErrMalformed, 2},
		{"bad nonce", []string{good[0], strings.Replace(good[1], `"nonce":"`, `"nonce":"AA`, 1)}, ErrMalformed, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "m.log")
			writeLines(t, p, tc.lines)
			_, err := Verify(p, nil)
			wantLineErr(t, err, tc.kind, tc.line)
		})
	}
	if _, err := Verify(filepath.Join(t.TempDir(), "missing"), nil); err == nil {
		t.Fatal("missing file")
	}
}

func TestOpenWriterValidation(t *testing.T) {
	dir := t.TempDir()
	if _, err := OpenWriter(filepath.Join(dir, "a.log"), testRun, []byte("short")); err == nil {
		t.Fatal("short master accepted")
	}
	if _, err := OpenWriter(filepath.Join(dir, "a.log"), "", GenerateMaster()); err == nil {
		t.Fatal("empty run id accepted")
	}
	if _, err := OpenWriter(filepath.Join(dir, "a.log"), "a\nb", GenerateMaster()); err == nil {
		t.Fatal("run id with newline accepted")
	}
	if _, err := OpenWriter(filepath.Join(dir, "nodir", "a.log"), testRun, GenerateMaster()); err == nil {
		t.Fatal("missing directory accepted")
	}
	var _ model.Auditor = (*Writer)(nil)
}

func TestConcurrentRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	master := GenerateMaster()
	w, err := OpenWriter(path, testRun, master)
	if err != nil {
		t.Fatal(err)
	}
	const g, per = 8, 25
	var wg sync.WaitGroup
	seqs := make(chan uint64, g*per)
	for i := 0; i < g; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < per; j++ {
				seq, err := w.Record(context.Background(), sampleRecord(i*per+j))
				if err != nil {
					t.Error(err)
					return
				}
				seqs <- seq
			}
		}(i)
	}
	wg.Wait()
	close(seqs)
	seen := map[uint64]bool{}
	for s := range seqs {
		if seen[s] {
			t.Fatalf("duplicate seq %d", s)
		}
		seen[s] = true
	}
	if len(seen) != g*per {
		t.Fatalf("%d seqs", len(seen))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	sum, err := Verify(path, master)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Records != g*per {
		t.Fatalf("records %d", sum.Records)
	}
}

func TestMasterParsing(t *testing.T) {
	k := GenerateMaster()
	for name, enc := range map[string]string{
		"std b64":  EncodeMaster(k),
		"raw b64":  base64.RawStdEncoding.EncodeToString(k),
		"hex":      strings.ToUpper(hexOf(k)),
		"with ws":  "  " + EncodeMaster(k) + "\n",
		"from env": EncodeMaster(k),
	} {
		t.Run(name, func(t *testing.T) {
			var got []byte
			var err error
			if name == "from env" {
				t.Setenv("ASSAY_TEST_MASTER", enc)
				got, err = MasterFromEnv("ASSAY_TEST_MASTER")
			} else {
				got, err = ParseMaster(enc)
			}
			if err != nil || !bytes.Equal(got, k) {
				t.Fatalf("%v %x", err, got)
			}
		})
	}
	for _, bad := range []string{"", "!!!", base64.StdEncoding.EncodeToString([]byte("short"))} {
		if _, err := ParseMaster(bad); err == nil {
			t.Fatalf("ParseMaster(%q) accepted", bad)
		}
	}
	if _, err := MasterFromEnv(""); err == nil {
		t.Fatal("empty name")
	}
	t.Setenv("ASSAY_TEST_EMPTY", "")
	if _, err := MasterFromEnv("ASSAY_TEST_EMPTY"); err == nil {
		t.Fatal("empty var")
	}
}

func hexOf(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, 0, len(b)*2)
	for _, c := range b {
		out = append(out, digits[c>>4], digits[c&0xf])
	}
	return string(out)
}
