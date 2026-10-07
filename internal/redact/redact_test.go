package redact

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

// Ported from tasia TestRedactNeverLeaks: a structured payload with a secret
// placed in a free-text field never reaches the output.
func TestRedactNeverLeaks(t *testing.T) {
	payload := `{"decision":"BLOCKED","risk":"HIGH","findings":[{"id":"x","evidence":"port=11434 key=sk-ABCDEFGH12345678 hf_realtokenvalue"}]}`
	p := Sanitize(payload)
	if strings.Contains(p, "sk-") || strings.Contains(p, "hf_real") {
		t.Errorf("redact leaked: %s", p)
	}
	if !strings.Contains(p, "BLOCKED") || !strings.Contains(p, "port=11434") {
		t.Errorf("redact removed benign structure: %s", p)
	}
}

// Ported from tasia TestSanitizeScrubsCommonTokenFormats, with the new
// pattern families appended.
func TestSanitizeScrubsCommonTokenFormats(t *testing.T) {
	cases := []struct {
		name string
		in   string
		// eatsTail: a truncated PEM block has no safe boundary, so the
		// pattern consumes the rest of the input on purpose.
		eatsTail bool
	}{
		{"openai style", "sk-ABCDEFGH12345678", false},
		{"anthropic style", "sk-ant-api03-ABCDEFGH12345678abcdefgh", false},
		{"huggingface", "hf_ABCDEFGH12345678", false},
		{"github pat", "ghp_0123456789ABCDEFGHIJ0123456789ab", false},
		{"aws access key id", "AKIAIOSFODNN7EXAMPLE", false},
		{"google api key", "AIzaSyA0123456789abcdefghijklmnopqrs", false},
		{"slack bot token", "xoxb-123456789012-abcdefghij", false},
		{"url credentials", "https://admin:p4ssw0rd@internal.host", false},
		{"authorization bearer", "Authorization: Bearer abcdef123456", false},
		{"authorization bearer lowercase", "authorization: bearer eyJabc.def.ghi", false},
		{"authorization basic", "Authorization: Basic YWRtaW46aHVudGVyMg==", false},
		{"x-api-key header", "x-api-key: 0123456789abcdef", false},
		{"X-Api-Key header mixed case", "X-Api-Key: abc-DEF_ghi.jkl", false},
		{"password kv", "password=hunter2xyz", false},
		{"client_secret kv", "client_secret: 9f8e7d6c5b4a", false},
		{"jwt", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2lnbmF0dXJl", false},
		{"pem full block", "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBg\nkqhkiG9w0B\n-----END PRIVATE KEY-----", false},
		{"pem rsa block", "-----BEGIN RSA PRIVATE KEY-----\nMIIEvQIBADANBg\n-----END RSA PRIVATE KEY-----", false},
		{"pem ec header only", "-----BEGIN EC PRIVATE KEY-----\nMIIEvQIBADANBg", true},
		{"pem openssh block", "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXk\n-----END OPENSSH PRIVATE KEY-----", false},
		{"gcp service account json", `"private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBg\n-----END PRIVATE KEY-----\n"`, false},
		{"service account private_key_id", `"private_key_id": "0123456789abcdef0123456789abcdef01234567"`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Sanitize("evidence " + tc.in + " tail")
			if strings.Contains(got, tc.in) {
				t.Errorf("sanitize failed to redact %q -> %q", tc.in, got)
			}
			if !strings.Contains(got, Marker) {
				t.Errorf("sanitize did not mark redaction for %q -> %q", tc.in, got)
			}
			if !strings.HasPrefix(got, "evidence ") || (!tc.eatsTail && !strings.HasSuffix(got, " tail")) {
				t.Errorf("sanitize damaged surrounding text: %q", got)
			}
			if !ContainsSecret(tc.in) {
				t.Errorf("ContainsSecret(%q) = false", tc.in)
			}
		})
	}
}

// Ported from tasia TestSanitizeKeepsBenignEvidence, plus the values that
// the audit log legitimately carries: digests, ids, canaries.
func TestSanitizeKeepsBenignEvidence(t *testing.T) {
	cases := []string{
		"image=ollama/ollama:latest port=11434:11434",
		"OLLAMA_ORIGINS=*",
		"OPENAI_API_KEY",
		"service=ollama privileged=true",
		"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		"ZDR-CANARY-0123456789abcdef",
		"run-2026-10-07T00:00:00Z",
		"GET /api/users/{id} 200",
		"https://lab.internal:8443/path",
		"Authorization header missing",
		"x-api-key header present",
		"tier=high capabilities=http.read,auth.login",
		"password reset endpoint lacks rate limit",
		"token count 1234",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if got := Sanitize(in); got != in {
				t.Errorf("sanitize should not alter benign evidence %q -> %q", in, got)
			}
			if ContainsSecret(in) {
				t.Errorf("ContainsSecret(%q) = true", in)
			}
		})
	}
}

func TestWriterIsLineBufferedAndSanitizes(t *testing.T) {
	var out bytes.Buffer
	w := Writer(&out)
	if _, err := w.Write([]byte("start sk-ABCDEF")); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("partial line should be buffered, got %q", out.String())
	}
	if _, err := w.Write([]byte("GH12345678 end\nclean line\n")); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "start "+Marker+" end\nclean line\n" {
		t.Fatalf("got %q", got)
	}
	// A split secret must be redacted once reassembled; a trailing partial
	// line is flushed on Close.
	if _, err := w.Write([]byte("tail AKIAIOSFODNN7EXAMPLE")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(out.String(), "tail "+Marker) {
		t.Fatalf("flush did not sanitize: %q", out.String())
	}
}

func TestWriterRedactsMultilinePEM(t *testing.T) {
	var out bytes.Buffer
	w := Writer(&out)
	lines := []string{
		"before\n",
		"-----BEGIN RSA PRIVATE KEY-----\n",
		"MIIEowIBAAKCAQEA0Z3VS5JJcds3xfn\n",
		"/ltHhsDiVJc2iJLl3m5x7S7ULPhXW8X\n",
		"-----END RSA PRIVATE KEY-----\n",
		"after\n",
	}
	for _, l := range lines {
		if _, err := w.Write([]byte(l)); err != nil {
			t.Fatal(err)
		}
	}
	got := out.String()
	for _, body := range []string{"MIIEowIBAAKCAQEA0Z3VS5JJcds3xfn", "ltHhsDiVJc2iJLl3m5x7S7ULPhXW8X", "BEGIN RSA", "END RSA"} {
		if strings.Contains(got, body) {
			t.Fatalf("PEM body leaked: %q", got)
		}
	}
	if !strings.HasPrefix(got, "before\n") || !strings.HasSuffix(got, "after\n") {
		t.Fatalf("surrounding lines damaged: %q", got)
	}
	if strings.Count(got, Marker) != 4 {
		t.Fatalf("expected 4 redacted lines, got %q", got)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("sink closed") }

func TestWriterPropagatesErrors(t *testing.T) {
	w := Writer(failWriter{})
	if _, err := w.Write([]byte("x\n")); err == nil {
		t.Fatal("expected error from sink")
	}
}

func TestHandlerSanitizesMessageAndAttrs(t *testing.T) {
	var out bytes.Buffer
	base := slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(Handler(base)).With("preset", "token=abcdef123456")
	logger.Info("call with sk-ABCDEFGH12345678",
		"hdr", "Authorization: Bearer abcdef123456",
		slog.Group("nested", slog.String("pw", "password=hunter2xyz"), slog.Int("n", 3)),
		"err", errors.New("dial https://u:p4ssw0rd@host failed"),
		"clean", "status=200",
	)
	logger.WithGroup("g").Warn("grouped", "k", "AKIAIOSFODNN7EXAMPLE")
	got := out.String()
	for _, leak := range []string{"sk-ABCDEFGH", "abcdef123456", "hunter2xyz", "p4ssw0rd", "AKIAIOSFODNN7EXAMPLE"} {
		if strings.Contains(got, leak) {
			t.Fatalf("handler leaked %q in %s", leak, got)
		}
	}
	for _, keep := range []string{"call with", "status=200", `"n":3`, "grouped", "preset"} {
		if !strings.Contains(got, keep) {
			t.Fatalf("handler dropped %q in %s", keep, got)
		}
	}
	if !logger.Enabled(t.Context(), slog.LevelDebug) {
		t.Fatal("Enabled should delegate")
	}
}
