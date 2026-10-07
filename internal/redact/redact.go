// Package redact removes credential-shaped substrings from text before it
// reaches a log, an audit record or a report.
//
// The pattern set is a defense-in-depth net. By construction the rest of the
// system only passes digests and bounded metadata to these sinks; redact
// catches whatever slips through. Patterns ported from tasia
// (internal/llm/redact.go, Apache-2.0, same owner) and extended.
package redact

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"sync"
)

// Marker replaces every matched secret.
const Marker = "[REDACTED]"

var secretPatterns = []*regexp.Regexp{
	// Vendor API keys with a recognizable prefix.
	regexp.MustCompile(`sk-ant-[A-Za-z0-9_-]{8,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{8,}`),
	regexp.MustCompile(`hf_[A-Za-z0-9]{8,}`),
	regexp.MustCompile(`gh[posru]_[A-Za-z0-9]{16,}`),
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`AIza[0-9A-Za-z_-]{20,}`),
	// JWT: three base64url segments.
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`),
	// PEM private keys: full block, or a lone header when the block was cut.
	regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----(?s:.*?)(?:-----END [A-Z0-9 ]*PRIVATE KEY-----|\z)`),
	// Cloud service-account JSON: the private_key member (and its id).
	regexp.MustCompile(`(?i)"private_key(?:_id)?"\s*:\s*"(?:[^"\\]|\\.)*"`),
	// Credentials embedded in URLs.
	regexp.MustCompile(`(?i)://[^/\s:@]+:[^/\s@]+@`),
	// HTTP credential headers.
	regexp.MustCompile(`(?i)\bx-api-key\s*[:=]\s*[A-Za-z0-9._~+/=-]{6,}`),
	regexp.MustCompile(`(?i)\bauthorization\s*[:=]\s*(?:bearer|basic|token)\s+[A-Za-z0-9._~+/=-]{6,}`),
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{6,}`),
	// Generic key=value credentials.
	regexp.MustCompile(`(?i)\b(?:token|secret|password|passwd|apikey|api_key|client_secret|access_key|private_key)\s*[:=]\s*\S{6,}`),
}

// pemHeader starts a multi-line PEM private key block; the line writer uses
// it to keep redacting until the matching footer.
var (
	pemHeader = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)
	pemFooter = regexp.MustCompile(`-----END [A-Z0-9 ]*PRIVATE KEY-----`)
)

// Sanitize replaces every credential-shaped substring of s with Marker.
func Sanitize(s string) string {
	for _, re := range secretPatterns {
		s = re.ReplaceAllString(s, Marker)
	}
	return s
}

// ContainsSecret reports whether s has at least one credential-shaped
// substring.
func ContainsSecret(s string) bool {
	for _, re := range secretPatterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}

// LineWriter is a line-buffered io.Writer that sanitizes each complete line
// before forwarding it. Close flushes a trailing partial line.
type LineWriter struct {
	mu    sync.Mutex
	w     io.Writer
	buf   bytes.Buffer
	inPEM bool
}

// Writer wraps w so that every line written through it is sanitized. The
// returned writer is also an io.Closer; Close flushes any buffered partial
// line (sanitized).
func Writer(w io.Writer) *LineWriter {
	return &LineWriter{w: w}
}

// Write buffers p and forwards complete lines through Sanitize.
func (l *LineWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Write(p)
	for {
		i := bytes.IndexByte(l.buf.Bytes(), '\n')
		if i < 0 {
			return len(p), nil
		}
		line := make([]byte, i+1)
		copy(line, l.buf.Next(i+1))
		if err := l.emit(line); err != nil {
			return 0, err
		}
	}
}

func (l *LineWriter) emit(line []byte) error {
	s := string(line)
	switch {
	case l.inPEM:
		if pemFooter.MatchString(s) {
			l.inPEM = false
		}
		s = Marker + "\n"
	case pemHeader.MatchString(s) && !pemFooter.MatchString(s):
		l.inPEM = true
		s = Marker + "\n"
	default:
		s = Sanitize(s)
	}
	_, err := io.WriteString(l.w, s)
	return err
}

// Close flushes a buffered partial line.
func (l *LineWriter) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buf.Len() == 0 {
		return nil
	}
	rest := l.buf.String()
	l.buf.Reset()
	if l.inPEM {
		rest = Marker
	} else {
		rest = Sanitize(rest)
	}
	_, err := io.WriteString(l.w, rest)
	return err
}

// Flush is an alias for Close that reads better at call sites that keep
// writing afterwards.
func (l *LineWriter) Flush() error { return l.Close() }

// handler sanitizes messages and attribute values before delegating.
type handler struct {
	next slog.Handler
}

// Handler wraps next so that every record's message and every string-valued
// attribute (including those added with WithAttrs and inside groups) is
// sanitized before next sees it.
func Handler(next slog.Handler) slog.Handler {
	return &handler{next: next}
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	clean := slog.NewRecord(r.Time, r.Level, Sanitize(r.Message), r.PC)
	r.Attrs(func(a slog.Attr) bool {
		clean.AddAttrs(sanitizeAttr(a))
		return true
	})
	return h.next.Handle(ctx, clean)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = sanitizeAttr(a)
	}
	return &handler{next: h.next.WithAttrs(out)}
}

func (h *handler) WithGroup(name string) slog.Handler {
	return &handler{next: h.next.WithGroup(name)}
}

func sanitizeAttr(a slog.Attr) slog.Attr {
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, Sanitize(v.String()))
	case slog.KindGroup:
		g := v.Group()
		out := make([]any, 0, len(g))
		for _, ga := range g {
			out = append(out, sanitizeAttr(ga))
		}
		return slog.Group(a.Key, out...)
	case slog.KindAny:
		// Errors, stringers and arbitrary values are flattened to text so the
		// patterns can see them; a value that is clean is kept as-is.
		s := fmt.Sprint(v.Any())
		if ContainsSecret(s) {
			return slog.String(a.Key, Sanitize(s))
		}
		return a
	default:
		return a
	}
}
