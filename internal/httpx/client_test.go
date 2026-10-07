package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

// fakeGate allows exactly the hosts in allow; it also restricts methods and
// rate-limits when configured.
type fakeGate struct {
	allow     map[string]bool
	methods   map[string]bool
	takeErr   error
	takeCalls int
}

func (g *fakeGate) Allow(_ context.Context, target string) model.Decision {
	u, err := url.Parse(target)
	if err != nil || !g.allow[u.Host] {
		return model.Decision{Effect: model.EffectDeny, Reason: "OUT_OF_SCOPE", Rationale: "host not authorized"}
	}
	return model.Decision{Effect: model.EffectAllow, Reason: "IN_SCOPE", PolicyHash: "fp"}
}

func (g *fakeGate) AllowMethod(m string) model.Decision {
	if g.methods == nil || g.methods[m] {
		return model.Decision{Effect: model.EffectAllow, Reason: "METHOD_ALLOWED"}
	}
	return model.Decision{Effect: model.EffectDeny, Reason: "METHOD_NOT_ALLOWED", Rationale: m}
}

func (g *fakeGate) Take() error { g.takeCalls++; return g.takeErr }

func hostOf(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

func newClient(gate model.Gate, aud model.Auditor) *Client {
	return &Client{Gate: gate, Auditor: aud, RunID: "run-1", Agent: "tester", Timeout: 5 * time.Second}
}

func TestDoAllowedAndAudited(t *testing.T) {
	var gotAuth, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		b, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Test", "yes")
		w.WriteHeader(201)
		w.Write([]byte("echo:" + string(b)))
	}))
	defer srv.Close()
	gate := &fakeGate{allow: map[string]bool{hostOf(t, srv): true}}
	aud := &MemAuditor{}
	c := newClient(gate, aud)
	status, hdrs, body, err := c.Do(context.Background(), "post", srv.URL+"/x", map[string]string{"Authorization": "Bearer secret"}, []byte("hi"))
	if err != nil {
		t.Fatal(err)
	}
	if status != 201 || string(body) != "echo:hi" || hdrs["X-Test"] != "yes" {
		t.Fatalf("status=%d body=%q hdrs=%v", status, body, hdrs)
	}
	if gotAuth != "Bearer secret" || !strings.HasPrefix(gotUA, "assay/") {
		t.Fatalf("headers not forwarded: %q %q", gotAuth, gotUA)
	}
	if gate.takeCalls != 1 {
		t.Fatalf("Take called %d times", gate.takeCalls)
	}
	gd := aud.ByKind("gate_decision")
	tc := aud.ByKind("tool_call")
	if len(gd) != 1 || len(tc) != 1 {
		t.Fatalf("audit: %d gate, %d tool", len(gd), len(tc))
	}
	if gd[0].Meta["effect"] != "allow" || gd[0].RunID != "run-1" || gd[0].Agent != "tester" {
		t.Fatalf("gate record %+v", gd[0])
	}
	rec := tc[0]
	if rec.Meta["subkind"] != "http" || rec.Meta["status"] != 201 || rec.Tool != "http" {
		t.Fatalf("tool record %+v", rec)
	}
	for _, v := range rec.Hashes {
		if len(v) != 64 {
			t.Fatalf("hash %q not sha256", v)
		}
	}
	// The record must not contain the secret or the body.
	raw, _ := json.Marshal(rec)
	if bytes.Contains(raw, []byte("secret")) || bytes.Contains(raw, []byte("echo:hi")) {
		t.Fatalf("audit record leaks content: %s", raw)
	}
	if tc[0].ToolCallID != gd[0].ToolCallID || !strings.HasPrefix(tc[0].ToolCallID, "http-") {
		t.Fatal("tool call ids must correlate gate and tool records")
	}
}

func TestRequestDigestRedactsSecrets(t *testing.T) {
	a := RequestDigest("GET", "http://h/", map[string]string{"Authorization": "a", "Cookie": "c1", "X-Other": "1"})
	b := RequestDigest("GET", "http://h/", map[string]string{"authorization": "b", "cookie": "c2", "X-Other": "1"})
	c := RequestDigest("GET", "http://h/", map[string]string{"Authorization": "a", "X-Other": "2"})
	if a != b {
		t.Fatal("secret header values must not affect the digest")
	}
	if a == c {
		t.Fatal("non-secret headers must affect the digest")
	}
}

func TestDeniedNeverSendsRequest(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()
	aud := &MemAuditor{}
	c := newClient(&fakeGate{allow: map[string]bool{}}, aud)
	_, _, _, err := c.Do(context.Background(), "GET", srv.URL, nil, nil)
	if !errors.Is(err, ErrDenied) {
		t.Fatalf("want ErrDenied, got %v", err)
	}
	var de *DeniedError
	if !errors.As(err, &de) || de.Decision.Reason != "OUT_OF_SCOPE" {
		t.Fatalf("decision not carried: %v", err)
	}
	if hit {
		t.Fatal("request was sent despite denial")
	}
	if gd := aud.ByKind("gate_decision"); len(gd) != 1 || gd[0].Meta["effect"] != "deny" {
		t.Fatalf("denial not audited: %+v", gd)
	}
	if len(aud.ByKind("tool_call")) != 0 {
		t.Fatal("no tool_call must be recorded for a denied request")
	}
}

func TestMethodDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	gate := &fakeGate{allow: map[string]bool{hostOf(t, srv): true}, methods: map[string]bool{"GET": true}}
	c := newClient(gate, &MemAuditor{})
	if _, _, _, err := c.Do(context.Background(), "DELETE", srv.URL, nil, nil); !errors.Is(err, ErrDenied) {
		t.Fatalf("want denial, got %v", err)
	}
	if _, _, _, err := c.Do(context.Background(), "GET", srv.URL, nil, nil); err != nil {
		t.Fatalf("GET: %v", err)
	}
}

func TestRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	limited := errors.New("RATE_LIMITED")
	gate := &fakeGate{allow: map[string]bool{hostOf(t, srv): true}, takeErr: limited}
	aud := &MemAuditor{}
	c := newClient(gate, aud)
	if _, _, _, err := c.Do(context.Background(), "GET", srv.URL, nil, nil); !errors.Is(err, limited) {
		t.Fatalf("got %v", err)
	}
	if len(aud.ByKind("gate_decision")) != 2 {
		t.Fatal("rate denial must be audited")
	}
}

func TestNilGateOrAuditorRefuses(t *testing.T) {
	c := &Client{Auditor: &MemAuditor{}}
	if _, _, _, err := c.Do(context.Background(), "GET", "http://127.0.0.1:1/", nil, nil); !errors.Is(err, ErrNoGate) {
		t.Fatalf("nil gate: %v", err)
	}
	c = &Client{Gate: &fakeGate{allow: map[string]bool{"127.0.0.1:1": true}}}
	if _, _, _, err := c.Do(context.Background(), "GET", "http://127.0.0.1:1/", nil, nil); !errors.Is(err, ErrNoAuditor) {
		t.Fatalf("nil auditor: %v", err)
	}
	var nilClient *Client
	if _, _, _, err := nilClient.Do(context.Background(), "GET", "http://127.0.0.1:1/", nil, nil); !errors.Is(err, ErrNoGate) {
		t.Fatalf("nil client: %v", err)
	}
}

func TestAuditFailureRefusesRequest(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()
	aud := &MemAuditor{FailOn: func(model.AuditRecord) error { return errors.New("disk full") }}
	c := newClient(&fakeGate{allow: map[string]bool{hostOf(t, srv): true}}, aud)
	if _, _, _, err := c.Do(context.Background(), "GET", srv.URL, nil, nil); err == nil || !strings.Contains(err.Error(), "audit failed") {
		t.Fatalf("got %v", err)
	}
	if hit {
		t.Fatal("request sent although audit failed")
	}
}

func TestBodyCapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("a"), 5000))
	}))
	defer srv.Close()
	aud := &MemAuditor{}
	c := newClient(&fakeGate{allow: map[string]bool{hostOf(t, srv): true}}, aud)
	c.MaxBody = 1000
	r, err := c.Exchange(context.Background(), "GET", srv.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Body) != 1000 || !r.Truncated {
		t.Fatalf("len=%d truncated=%v", len(r.Body), r.Truncated)
	}
	if tc := aud.ByKind("tool_call"); tc[0].Meta["truncated"] != true || r.AuditSeq != tc[0].Seq {
		t.Fatalf("audit meta %+v seq %d", tc[0].Meta, r.AuditSeq)
	}
}

func TestSameHostRedirectFollowedAndRechecked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/start":
			http.Redirect(w, r, "/end", http.StatusFound)
		case "/end":
			if r.Header.Get("Authorization") != "" {
				w.WriteHeader(500)
				return
			}
			w.Write([]byte("done"))
		}
	}))
	defer srv.Close()
	aud := &MemAuditor{}
	c := newClient(&fakeGate{allow: map[string]bool{hostOf(t, srv): true}}, aud)
	status, _, body, err := c.Do(context.Background(), "GET", srv.URL+"/start", map[string]string{"Authorization": "x"}, nil)
	if err != nil || status != 200 || string(body) != "done" {
		t.Fatalf("status=%d body=%q err=%v", status, body, err)
	}
	gd := aud.ByKind("gate_decision")
	if len(gd) != 2 || gd[1].Meta["stage"] != "redirect" {
		t.Fatalf("redirect target not re-checked: %+v", gd)
	}
}

func TestCrossHostRedirectRefused(t *testing.T) {
	hit := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/evil", http.StatusFound)
	}))
	defer srv.Close()
	aud := &MemAuditor{}
	// Even if the gate would allow the other host, cross-host is refused.
	c := newClient(&fakeGate{allow: map[string]bool{hostOf(t, srv): true, hostOf(t, other): true}}, aud)
	_, _, _, err := c.Do(context.Background(), "GET", srv.URL+"/start", nil, nil)
	if !errors.Is(err, ErrCrossHostRedirect) {
		t.Fatalf("want ErrCrossHostRedirect, got %v", err)
	}
	if hit {
		t.Fatal("redirect target was fetched")
	}
}

func TestRedirectToDisallowedHostRefused(t *testing.T) {
	hit := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/evil", http.StatusFound)
	}))
	defer srv.Close()
	aud := &MemAuditor{}
	c := newClient(&fakeGate{allow: map[string]bool{hostOf(t, srv): true}}, aud)
	_, _, _, err := c.Do(context.Background(), "GET", srv.URL+"/start", nil, nil)
	if err == nil {
		t.Fatal("expected refusal")
	}
	if hit {
		t.Fatal("disallowed redirect target was fetched")
	}
	// The failed exchange is still audited.
	if len(aud.ByKind("tool_call")) != 1 {
		t.Fatal("failed exchange must be audited")
	}
}

func TestDoNoFollowReturnsLocation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://assay-marker.invalid/", http.StatusFound)
	}))
	defer srv.Close()
	c := newClient(&fakeGate{allow: map[string]bool{hostOf(t, srv): true}}, &MemAuditor{})
	status, hdrs, _, err := c.DoNoFollow(context.Background(), "GET", srv.URL+"/r", nil, nil)
	if err != nil || status != 302 || hdrs["Location"] != "http://assay-marker.invalid/" {
		t.Fatalf("status=%d hdrs=%v err=%v", status, hdrs, err)
	}
}

func TestTooManyRedirects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer srv.Close()
	c := newClient(&fakeGate{allow: map[string]bool{hostOf(t, srv): true}}, &MemAuditor{})
	if _, _, _, err := c.Do(context.Background(), "GET", srv.URL+"/loop", nil, nil); !errors.Is(err, ErrTooManyRedirects) {
		t.Fatalf("got %v", err)
	}
}

func TestWriterAuditorChains(t *testing.T) {
	var buf bytes.Buffer
	a := &WriterAuditor{W: &buf}
	s1, _ := a.Record(context.Background(), model.AuditRecord{Kind: "x"})
	s2, _ := a.Record(context.Background(), model.AuditRecord{Kind: "y"})
	if s1 != 1 || s2 != 2 {
		t.Fatal("sequence")
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"prev_hash":""`) || strings.Contains(lines[1], `"prev_hash":""`) {
		t.Fatalf("chain: %v", lines)
	}
}
