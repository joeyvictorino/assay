// Package httpx is the only HTTP client in assay. Every outbound request is
// first put to the authorization Gate and then audited with digests only.
//
// Redirects are followed only within the same host and only after the
// redirect target has itself been allowed by the Gate. Response bodies are
// capped. A nil Gate or nil Auditor refuses every request (fail closed).
package httpx

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

// Sentinel errors. Use errors.Is; the Decision travels in *DeniedError.
var (
	ErrDenied            = errors.New("httpx: request denied by gate")
	ErrNoGate            = errors.New("httpx: no gate configured; refusing request")
	ErrNoAuditor         = errors.New("httpx: no auditor configured; refusing request")
	ErrCrossHostRedirect = errors.New("httpx: redirect to a different host refused")
	ErrTooManyRedirects  = errors.New("httpx: too many redirects")
)

// DeniedError carries the gate Decision for a refused request.
type DeniedError struct {
	URL      string
	Decision model.Decision
}

func (e *DeniedError) Error() string {
	return fmt.Sprintf("httpx: gate denied %s: %s: %s", e.URL, e.Decision.Reason, e.Decision.Rationale)
}

// Is makes errors.Is(err, ErrDenied) true.
func (e *DeniedError) Is(target error) bool { return target == ErrDenied }

// MethodGate is implemented by gates that also restrict HTTP methods.
type MethodGate interface {
	AllowMethod(method string) model.Decision
}

// RateGate is implemented by gates that enforce a request rate.
type RateGate interface {
	Take() error
}

// DefaultMaxBody caps response bodies when the Client does not set MaxBody.
const DefaultMaxBody int64 = 1 << 20

// maxRedirects bounds same-host redirect chains.
const maxRedirects = 5

// Client is the gated, audited HTTP client.
type Client struct {
	Gate      model.Gate
	Auditor   model.Auditor
	RunID     string
	Agent     string
	Timeout   time.Duration
	MaxBody   int64
	UserAgent string

	// Transport overrides the default transport (tests).
	Transport http.RoundTripper
}

// Response is the full result of one exchange, with the digests the audit
// log recorded so callers can cite them as Evidence.
type Response struct {
	Status         int
	Headers        map[string]string
	Body           []byte
	Truncated      bool
	RequestDigest  string
	BodyDigest     string
	ResponseDigest string
	ToolCallID     string
	AuditSeq       uint64
	FinalURL       string
	Elapsed        time.Duration
}

// Do performs a gated, audited request and follows same-host redirects.
func (c *Client) Do(ctx context.Context, method, target string, headers map[string]string, body []byte) (int, map[string]string, []byte, error) {
	r, err := c.do(ctx, method, target, headers, body, true)
	if err != nil {
		return 0, nil, nil, err
	}
	return r.Status, r.Headers, r.Body, nil
}

// DoNoFollow performs a gated, audited request without following any
// redirect, returning the 3xx response and its Location header untouched.
// The Location target is never fetched.
func (c *Client) DoNoFollow(ctx context.Context, method, target string, headers map[string]string, body []byte) (int, map[string]string, []byte, error) {
	r, err := c.do(ctx, method, target, headers, body, false)
	if err != nil {
		return 0, nil, nil, err
	}
	return r.Status, r.Headers, r.Body, nil
}

// Exchange is Do returning the full Response (digests, audit seq).
func (c *Client) Exchange(ctx context.Context, method, target string, headers map[string]string, body []byte) (*Response, error) {
	return c.do(ctx, method, target, headers, body, true)
}

// ExchangeNoFollow is DoNoFollow returning the full Response.
func (c *Client) ExchangeNoFollow(ctx context.Context, method, target string, headers map[string]string, body []byte) (*Response, error) {
	return c.do(ctx, method, target, headers, body, false)
}

func (c *Client) do(ctx context.Context, method, target string, headers map[string]string, body []byte, follow bool) (*Response, error) {
	if c == nil || c.Gate == nil {
		return nil, ErrNoGate
	}
	if c.Auditor == nil {
		return nil, ErrNoAuditor
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		method = http.MethodGet
	}
	toolCallID, err := newToolCallID()
	if err != nil {
		return nil, err
	}

	if err := c.gateCheck(ctx, toolCallID, method, target, "initial"); err != nil {
		return nil, err
	}
	if rg, ok := c.Gate.(RateGate); ok {
		if err := rg.Take(); err != nil {
			_, _ = c.audit(ctx, model.AuditRecord{
				Kind:       "gate_decision",
				ToolCallID: toolCallID,
				Tool:       "http",
				Meta:       map[string]any{"stage": "rate", "url": target, "effect": "deny", "reason": "RATE_LIMITED"},
			})
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("httpx: build request: %w", err)
	}
	ua := c.UserAgent
	if ua == "" {
		ua = "assay/dev (authorized security validation)"
	}
	req.Header.Set("User-Agent", ua)
	for k, v := range headers {
		if strings.EqualFold(k, "Host") {
			req.Host = v
			continue
		}
		req.Header.Set(k, v)
	}

	maxBody := c.MaxBody
	if maxBody <= 0 {
		maxBody = DefaultMaxBody
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	origHost := req.URL.Host
	hc := &http.Client{
		Timeout:   timeout,
		Transport: c.Transport,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if !follow {
				return http.ErrUseLastResponse
			}
			if len(via) >= maxRedirects {
				return ErrTooManyRedirects
			}
			if r.URL.User != nil {
				return &DeniedError{URL: r.URL.String(), Decision: model.Decision{Effect: model.EffectDeny, Reason: "OUT_OF_SCOPE", Rationale: "redirect target carries userinfo"}}
			}
			if !strings.EqualFold(r.URL.Host, origHost) {
				_, _ = c.audit(ctx, model.AuditRecord{
					Kind:       "gate_decision",
					ToolCallID: toolCallID,
					Tool:       "http",
					Meta:       map[string]any{"stage": "redirect", "url": r.URL.String(), "effect": "deny", "reason": "CROSS_HOST_REDIRECT"},
				})
				return ErrCrossHostRedirect
			}
			if err := c.gateCheck(ctx, toolCallID, r.Method, r.URL.String(), "redirect"); err != nil {
				return err
			}
			// Never forward credentials across redirects.
			r.Header.Del("Authorization")
			r.Header.Del("Cookie")
			return nil
		},
	}

	reqDigest := RequestDigest(method, target, headers)
	bodyDigest := digest(body)
	start := time.Now()
	resp, err := hc.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		_, _ = c.audit(ctx, model.AuditRecord{
			Kind:       "tool_call",
			ToolCallID: toolCallID,
			Tool:       "http",
			ArgsDigest: reqDigest,
			Hashes:     map[string]string{"request": reqDigest, "body": bodyDigest},
			Meta:       map[string]any{"subkind": "http", "method": method, "host": origHost, "error": errClass(err), "elapsed_ms": elapsed.Milliseconds()},
		})
		return nil, err
	}
	defer resp.Body.Close()

	lr := io.LimitReader(resp.Body, maxBody+1)
	data, rerr := io.ReadAll(lr)
	truncated := false
	if int64(len(data)) > maxBody {
		data = data[:maxBody]
		truncated = true
	}
	respDigest := digest(data)
	respHeaders := flattenHeaders(resp.Header)

	seq, aerr := c.audit(ctx, model.AuditRecord{
		Kind:       "tool_call",
		ToolCallID: toolCallID,
		Tool:       "http",
		ArgsDigest: reqDigest,
		Hashes:     map[string]string{"request": reqDigest, "body": bodyDigest, "response": respDigest},
		Meta: map[string]any{
			"subkind":    "http",
			"method":     method,
			"host":       origHost,
			"status":     resp.StatusCode,
			"bytes":      len(data),
			"truncated":  truncated,
			"redirected": resp.Request.URL.String() != target,
			"elapsed_ms": elapsed.Milliseconds(),
		},
	})
	if aerr != nil {
		return nil, fmt.Errorf("httpx: audit failed; result discarded: %w", aerr)
	}
	if rerr != nil {
		return nil, fmt.Errorf("httpx: read body: %w", rerr)
	}
	return &Response{
		Status:         resp.StatusCode,
		Headers:        respHeaders,
		Body:           data,
		Truncated:      truncated,
		RequestDigest:  reqDigest,
		BodyDigest:     bodyDigest,
		ResponseDigest: respDigest,
		ToolCallID:     toolCallID,
		AuditSeq:       seq,
		FinalURL:       resp.Request.URL.String(),
		Elapsed:        elapsed,
	}, nil
}

// gateCheck puts a URL (and method) to the Gate and audits the decision.
func (c *Client) gateCheck(ctx context.Context, toolCallID, method, target, stage string) error {
	d := c.Gate.Allow(ctx, target)
	if d.Effect == model.EffectAllow {
		if mg, ok := c.Gate.(MethodGate); ok {
			if md := mg.AllowMethod(method); md.Effect != model.EffectAllow {
				d = md
			}
		}
	}
	if d.Effect != model.EffectAllow {
		// Deny wins, including an empty Effect.
		if d.Effect == "" {
			d.Effect = model.EffectDeny
			if d.Reason == "" {
				d.Reason = "GATE_EMPTY_DECISION"
			}
		}
	}
	if _, err := c.audit(ctx, model.AuditRecord{
		Kind:       "gate_decision",
		ToolCallID: toolCallID,
		Tool:       "http",
		ArgsDigest: digest([]byte(method + "\n" + target)),
		Meta: map[string]any{
			"stage":       stage,
			"method":      method,
			"url":         target,
			"effect":      string(d.Effect),
			"reason":      d.Reason,
			"rationale":   d.Rationale,
			"policy_hash": d.PolicyHash,
		},
	}); err != nil {
		return fmt.Errorf("httpx: audit failed; refusing request: %w", err)
	}
	if d.Effect != model.EffectAllow {
		return &DeniedError{URL: target, Decision: d}
	}
	return nil
}

func (c *Client) audit(ctx context.Context, rec model.AuditRecord) (uint64, error) {
	rec.Time = time.Now().UTC()
	rec.RunID = c.RunID
	rec.Agent = c.Agent
	return c.Auditor.Record(ctx, rec)
}

// RequestDigest is sha256 over method, url and the sorted request headers
// with Authorization and Cookie values replaced by a fixed redaction token.
func RequestDigest(method, target string, headers map[string]string) string {
	canon := make(map[string]string, len(headers))
	keys := make([]string, 0, len(headers))
	for k, v := range headers {
		ck := http.CanonicalHeaderKey(k)
		if _, dup := canon[ck]; !dup {
			keys = append(keys, ck)
		}
		canon[ck] = v
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(strings.ToUpper(method))
	b.WriteByte('\n')
	b.WriteString(target)
	b.WriteByte('\n')
	for _, k := range keys {
		v := canon[k]
		if isSecretHeader(k) {
			v = "<redacted>"
		}
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteByte('\n')
	}
	return digest([]byte(b.String()))
}

func isSecretHeader(k string) bool {
	switch http.CanonicalHeaderKey(k) {
	case "Authorization", "Cookie", "Proxy-Authorization", "X-Api-Key", "Set-Cookie":
		return true
	}
	return false
}

func digest(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func flattenHeaders(h http.Header) map[string]string {
	out := make(map[string]string, len(h))
	for k, vs := range h {
		out[http.CanonicalHeaderKey(k)] = strings.Join(vs, ", ")
	}
	return out
}

func newToolCallID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("httpx: tool call id: %w", err)
	}
	return "http-" + hex.EncodeToString(b[:]), nil
}

// errClass reduces a transport error to a bounded label for the audit log.
func errClass(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrDenied):
		return "denied"
	case errors.Is(err, ErrCrossHostRedirect):
		return "cross_host_redirect"
	case errors.Is(err, ErrTooManyRedirects):
		return "too_many_redirects"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return "timeout"
	}
	return "transport"
}
