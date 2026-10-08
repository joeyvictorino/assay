package validate

// Tests for the observational and session-based checkers: auth-bypass,
// jwt-weak, mass-assignment, xss-stored, rate-limit-missing, csrf and ssrf.
// Each runs against a purpose-built httptest handler in a vulnerable and a
// hardened form. Like validate_test.go, nothing here reaches a real lab.

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/labs"
	"github.com/joeyvictorino/assay/internal/model"
)

// webLab is a lab around an arbitrary handler. Login posts to /login and
// expects {"token","id"} back, like the mimic.
type webLab struct {
	srv    *httptest.Server
	client *httpx.Client
	gate   *recordingGate
}

func newWebLab(t *testing.T, h http.Handler) *webLab {
	t.Helper()
	w := &webLab{srv: httptest.NewServer(h)}
	t.Cleanup(w.srv.Close)
	u, _ := url.Parse(w.srv.URL)
	w.gate = &recordingGate{host: u.Host}
	w.client = &httpx.Client{Gate: w.gate, Auditor: &httpx.MemAuditor{}, RunID: "t", Timeout: 5 * time.Second}
	return w
}

func (w *webLab) Name() string                 { return "web" }
func (w *webLab) BaseURL() string              { return w.srv.URL }
func (w *webLab) Health(context.Context) error { return nil }
func (w *webLab) Setup(context.Context) error  { return nil }
func (w *webLab) Accounts() []labs.Account {
	return []labs.Account{{Role: "user", Username: "alice", Password: "Wonderland-1"}}
}
func (w *webLab) GroundTruth() ([]labs.GroundTruthEntry, bool) { return nil, false }
func (w *webLab) Login(ctx context.Context, acct labs.Account) (map[string]string, string, error) {
	body, _ := json.Marshal(map[string]string{"username": acct.Username, "password": acct.Password})
	status, _, resp, err := w.client.Do(ctx, "POST", w.srv.URL+"/login", map[string]string{"Content-Type": "application/json"}, body)
	if err != nil {
		return nil, "", err
	}
	if status == 401 {
		return nil, "", fmt.Errorf("rejected: %w", labs.ErrLoginRejected)
	}
	var out struct{ Token, ID string }
	_ = json.Unmarshal(resp, &out)
	return map[string]string{"Authorization": "Bearer " + out.Token}, out.ID, nil
}

func (w *webLab) env() Env {
	return Env{HTTP: w.client, Lab: w, Pace: -1,
		Now: func() time.Time { return time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC) }}
}

func (w *webLab) check(t *testing.T, f model.Finding) model.Finding {
	t.Helper()
	out := Validate(context.Background(), []model.Finding{f}, w.env())
	if out[0].Validation == nil || out[0].Validation.Checker == "none" {
		t.Fatalf("%s: no checker ran: %+v", f.Class, out[0].Validation)
	}
	return out[0]
}

func (w *webLab) assertSameHostOnly(t *testing.T) {
	t.Helper()
	u, _ := url.Parse(w.srv.URL)
	w.gate.mu.Lock()
	defer w.gate.mu.Unlock()
	for _, target := range w.gate.urls {
		p, _ := url.Parse(target)
		if p.Host != u.Host {
			t.Fatalf("request left the lab: %s", target)
		}
	}
}

func loginHandler(token string) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		var in struct{ Username, Password string }
		_ = json.NewDecoder(r.Body).Decode(&in)
		if in.Password != "Wonderland-1" {
			http.Error(rw, "no", http.StatusUnauthorized)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(rw).Encode(map[string]string{"token": token, "id": "2"})
	}
}

func extra(id string, class model.Class, method, path, param string) model.Finding {
	return model.Finding{ID: id, Lab: "web", Class: class, State: model.StateTheorized,
		Location: model.Location{Method: method, PathTemplate: path, Param: param}, Severity: model.SevMedium}
}

// ---- rate-limit-missing ----------------------------------------------------

func TestRateLimitMissing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		limitAt int32 // 0 = never limited
		want    model.FindingState
	}{
		{"never limited", 0, model.StateValidated},
		{"limited from the start", 1, model.StateRefuted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var n atomic.Int32
			mux := http.NewServeMux()
			mux.HandleFunc("/login", func(rw http.ResponseWriter, r *http.Request) {
				c := n.Add(1)
				if tc.limitAt > 0 && c >= tc.limitAt {
					rw.Header().Set("Retry-After", "30")
					http.Error(rw, "slow down", http.StatusTooManyRequests)
					return
				}
				http.Error(rw, "bad credentials", http.StatusUnauthorized)
			})
			w := newWebLab(t, mux)
			got := w.check(t, extra("RL", model.ClassRateLimit, "POST", "/login", "password"))
			if got.State != tc.want {
				t.Fatalf("state %s, want %s: %s", got.State, tc.want, got.Validation.Note)
			}
			if c := n.Load(); c > 20 {
				t.Fatalf("sent %d requests; the probe must stay at or below 20", c)
			}
			w.assertSameHostOnly(t)
		})
	}
}

func TestRateLimitLimitedInOneBatchOnlyStaysTheorized(t *testing.T) {
	var n atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(rw http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 3 { // only inside the first batch
			http.Error(rw, "slow", http.StatusTooManyRequests)
			return
		}
		http.Error(rw, "bad", http.StatusUnauthorized)
	})
	w := newWebLab(t, mux)
	if got := w.check(t, extra("RL", model.ClassRateLimit, "POST", "/login", "password")); got.State != model.StateTheorized {
		t.Fatalf("state %s", got.State)
	}
}

// ---- csrf ------------------------------------------------------------------

func TestCSRF(t *testing.T) {
	page := func(body string) http.Handler {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			rw.Header().Set("Content-Type", "text/html")
			io.WriteString(rw, body)
		})
	}
	for _, tc := range []struct {
		name string
		body string
		want model.FindingState
	}{
		{"post form without token", `<form method="post" action="/change"><input name="email"></form>`, model.StateValidated},
		{"post form with token", `<form method="post" action="/change"><input type="hidden" name="csrf_token" value="x"><input name="email"></form>`, model.StateRefuted},
		{"no form", `<p>nothing to post here</p>`, model.StateTheorized},
		{"get form only", `<form method="get" action="/s"><input name="q"></form>`, model.StateTheorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var posts atomic.Int32
			h := page(tc.body)
			w := newWebLab(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					posts.Add(1)
				}
				h.ServeHTTP(rw, r)
			}))
			got := w.check(t, extra("CS", model.ClassCSRF, "POST", "/account", ""))
			if got.State != tc.want {
				t.Fatalf("state %s, want %s: %s", got.State, tc.want, got.Validation.Note)
			}
			if posts.Load() != 0 {
				t.Fatalf("csrf check must be observation only, saw %d non-GET requests", posts.Load())
			}
		})
	}
}

func TestCSRFSameSiteCookieCountsAsProtection(t *testing.T) {
	w := newWebLab(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/html")
		rw.Header().Set("Set-Cookie", "sid=abc; Path=/; SameSite=Strict")
		io.WriteString(rw, `<form method="post" action="/change"><input name="email"></form>`)
	}))
	if got := w.check(t, extra("CS", model.ClassCSRF, "POST", "/account", "")); got.State != model.StateRefuted {
		t.Fatalf("state %s: %s", got.State, got.Validation.Note)
	}
}

// ---- ssrf ------------------------------------------------------------------

func TestSSRF(t *testing.T) {
	for _, vulnerable := range []bool{true, false} {
		name := map[bool]string{true: "vulnerable", false: "hardened"}[vulnerable]
		t.Run(name, func(t *testing.T) {
			var external atomic.Int32
			mux := http.NewServeMux()
			mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
				io.WriteString(rw, "Welcome to the assay training application landing page")
			})
			mux.HandleFunc("/fetch", func(rw http.ResponseWriter, r *http.Request) {
				target := r.URL.Query().Get("url")
				if !vulnerable {
					http.Error(rw, "url not allowed", http.StatusBadRequest)
					return
				}
				resp, err := http.Get(target)
				if err != nil {
					external.Add(1)
					http.Error(rw, err.Error(), http.StatusBadGateway)
					return
				}
				defer resp.Body.Close()
				io.Copy(rw, resp.Body)
			})
			w := newWebLab(t, mux)
			got := w.check(t, extra("SS", model.ClassSSRF, "GET", "/fetch", "url"))
			want := map[bool]model.FindingState{true: model.StateValidated, false: model.StateRefuted}[vulnerable]
			if got.State != want {
				t.Fatalf("state %s, want %s: %s", got.State, want, got.Validation.Note)
			}
			w.assertSameHostOnly(t)
		})
	}
}

func TestSSRFNeverProbesANonLoopbackLab(t *testing.T) {
	lab := &stubLab{base: "http://lab.example.test"}
	out := Validate(context.Background(), []model.Finding{extra("SS", model.ClassSSRF, "GET", "/fetch", "url")}, Env{HTTP: &httpx.Client{Gate: denyAll{}, Auditor: &httpx.MemAuditor{}, RunID: "t"}, Lab: lab})
	if out[0].State != model.StateTheorized || !strings.Contains(out[0].Validation.Note, "loopback") {
		t.Fatalf("%+v", out[0].Validation)
	}
}

type stubLab struct{ base string }

func (s *stubLab) Name() string                                 { return "stub" }
func (s *stubLab) BaseURL() string                              { return s.base }
func (s *stubLab) Health(context.Context) error                 { return nil }
func (s *stubLab) Setup(context.Context) error                  { return nil }
func (s *stubLab) Accounts() []labs.Account                     { return nil }
func (s *stubLab) GroundTruth() ([]labs.GroundTruthEntry, bool) { return nil, false }

type denyAll struct{}

func (denyAll) Allow(context.Context, string) model.Decision {
	return model.Decision{Effect: model.EffectDeny, Reason: "OUT_OF_SCOPE"}
}

// ---- auth-bypass -----------------------------------------------------------

func TestAuthBypass(t *testing.T) {
	for _, vulnerable := range []bool{true, false} {
		name := map[bool]string{true: "vulnerable", false: "hardened"}[vulnerable]
		t.Run(name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/" {
					http.NotFound(rw, r)
					return
				}
				io.WriteString(rw, "public home page of the training app")
			})
			mux.HandleFunc("/admin", func(rw http.ResponseWriter, r *http.Request) {
				if vulnerable && r.Header.Get("X-Forwarded-For") == "127.0.0.1" {
					io.WriteString(rw, "admin console: users, keys, settings")
					return
				}
				http.Error(rw, "forbidden", http.StatusForbidden)
			})
			w := newWebLab(t, mux)
			got := w.check(t, extra("AB", model.ClassAuthBypass, "GET", "/admin", ""))
			want := map[bool]model.FindingState{true: model.StateValidated, false: model.StateRefuted}[vulnerable]
			if got.State != want {
				t.Fatalf("state %s, want %s: %s", got.State, want, got.Validation.Note)
			}
			w.assertSameHostOnly(t)
		})
	}
}

func TestAuthBypassOpenResourceIsAuthMissingNotBypass(t *testing.T) {
	w := newWebLab(t, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		io.WriteString(rw, "open to everyone")
	}))
	got := w.check(t, extra("AB", model.ClassAuthBypass, "GET", "/admin", ""))
	if got.State != model.StateTheorized || !strings.Contains(got.Validation.Note, "auth-missing") {
		t.Fatalf("%s: %s", got.State, got.Validation.Note)
	}
}

func TestAuthBypassServerIgnoringHeaderIsNotABypass(t *testing.T) {
	// The protected path answers 200 only to the shaped request that is
	// identical with and without the header (a home page), so the control
	// request must stop it counting as a bypass.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin" {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		io.WriteString(rw, "public home page of the training app")
	})
	w := newWebLab(t, mux)
	if got := w.check(t, extra("AB", model.ClassAuthBypass, "GET", "/admin", "")); got.State == model.StateValidated {
		t.Fatalf("home page misread as a bypass: %s", got.Validation.Note)
	}
}

// ---- jwt-weak --------------------------------------------------------------

var jwtEvidenceRe = regexp.MustCompile(`^HS(256|384|512) signature verifies with default-list secret #\d+$`)

func signHS256(secret, claims string) string {
	enc := base64.RawURLEncoding.EncodeToString
	head := enc([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := enc([]byte(claims))
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(head + "." + payload))
	return head + "." + payload + "." + enc(m.Sum(nil))
}

func TestJWTWeak(t *testing.T) {
	strong := "this-secret-is-long-and-random-0123456789abcdef"
	for _, tc := range []struct {
		name       string
		secret     string
		acceptNone bool
		want       model.FindingState
	}{
		{"default secret", "secret", false, model.StateValidated},
		{"alg none accepted", strong, true, model.StateValidated},
		{"hardened", strong, false, model.StateRefuted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/login", loginHandler(signHS256(tc.secret, `{"sub":"2"}`)))
			mux.HandleFunc("/me", func(rw http.ResponseWriter, r *http.Request) {
				tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
				if strings.HasSuffix(tok, ".") && tc.acceptNone {
					io.WriteString(rw, `{"user":"alice"}`)
					return
				}
				if tok == signHS256(tc.secret, `{"sub":"2"}`) {
					io.WriteString(rw, `{"user":"alice"}`)
					return
				}
				http.Error(rw, "invalid token", http.StatusUnauthorized)
			})
			w := newWebLab(t, mux)
			got := w.check(t, extra("JW", model.ClassJWTWeak, "GET", "/me", ""))
			if got.State != tc.want {
				t.Fatalf("state %s, want %s: %s", got.State, tc.want, got.Validation.Note)
			}
			// Only the index into the built-in list may appear, never a
			// candidate value; the strong secret must not appear at all.
			for _, ev := range got.Validation.Evidence {
				if ev.Kind == "tool-output" && !jwtEvidenceRe.MatchString(ev.Marker) {
					t.Fatalf("unexpected evidence text: %q", ev.Marker)
				}
				if strings.Contains(ev.Marker, strong) {
					t.Fatalf("evidence leaks the secret: %q", ev.Marker)
				}
			}
			if strings.Contains(got.Validation.Note, strong) || strings.Contains(got.Validation.Note, `"secret"`) {
				t.Fatalf("note leaks a secret value: %s", got.Validation.Note)
			}
			w.assertSameHostOnly(t)
		})
	}
}

func TestJWTWeakNonJWTSessionStaysTheorized(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", loginHandler("opaque-session-token"))
	w := newWebLab(t, mux)
	if got := w.check(t, extra("JW", model.ClassJWTWeak, "GET", "/me", "")); got.State != model.StateTheorized {
		t.Fatalf("state %s", got.State)
	}
}

// ---- mass-assignment -------------------------------------------------------

func TestMassAssignment(t *testing.T) {
	for _, vulnerable := range []bool{true, false} {
		name := map[bool]string{true: "vulnerable", false: "hardened"}[vulnerable]
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			store := map[string]any{"name": "alice"}
			var wroteRole atomic.Int32
			mux := http.NewServeMux()
			mux.HandleFunc("/login", loginHandler("tok"))
			mux.HandleFunc("/users/", func(rw http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				rw.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPut {
					var in map[string]any
					_ = json.NewDecoder(r.Body).Decode(&in)
					if _, ok := in["role"]; ok {
						wroteRole.Add(1)
					}
					for k, v := range in {
						if vulnerable || k == "name" {
							store[k] = v
						}
					}
				}
				_ = json.NewEncoder(rw).Encode(store)
			})
			w := newWebLab(t, mux)
			got := w.check(t, extra("MA", model.ClassMassAssignment, "PUT", "/users/{id}", "role"))
			want := map[bool]model.FindingState{true: model.StateValidated, false: model.StateRefuted}[vulnerable]
			if got.State != want {
				t.Fatalf("state %s, want %s: %s", got.State, want, got.Validation.Note)
			}
			if wroteRole.Load() != 0 {
				t.Fatalf("the checker wrote the privileged field %d times", wroteRole.Load())
			}
			w.assertSameHostOnly(t)
		})
	}
}

func TestMassAssignmentGetFindingIsNotExercised(t *testing.T) {
	w := newWebLab(t, http.NotFoundHandler())
	if got := w.check(t, extra("MA", model.ClassMassAssignment, "GET", "/users/{id}", "role")); got.State != model.StateTheorized {
		t.Fatalf("state %s", got.State)
	}
}

// ---- xss-stored ------------------------------------------------------------

func TestXSSStored(t *testing.T) {
	for _, vulnerable := range []bool{true, false} {
		name := map[bool]string{true: "vulnerable", false: "hardened"}[vulnerable]
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			bio := ""
			var sawScript atomic.Int32
			mux := http.NewServeMux()
			mux.HandleFunc("/login", loginHandler("tok"))
			mux.HandleFunc("/profile", func(rw http.ResponseWriter, r *http.Request) {
				var in map[string]string
				_ = json.NewDecoder(r.Body).Decode(&in)
				if strings.Contains(strings.ToLower(in["bio"]), "<script") {
					sawScript.Add(1)
				}
				mu.Lock()
				bio = in["bio"]
				mu.Unlock()
				rw.Header().Set("Content-Type", "application/json")
				io.WriteString(rw, `{"ok":true,"view":"/profile/2"}`)
			})
			mux.HandleFunc("/profile/", func(rw http.ResponseWriter, r *http.Request) {
				mu.Lock()
				b := bio
				mu.Unlock()
				if !vulnerable {
					b = html.EscapeString(b)
				}
				rw.Header().Set("Content-Type", "text/html")
				io.WriteString(rw, "<html><body><div class=bio>"+b+"</div></body></html>")
			})
			w := newWebLab(t, mux)
			got := w.check(t, extra("XS", model.ClassXSSStored, "POST", "/profile", "bio"))
			want := map[bool]model.FindingState{true: model.StateValidated, false: model.StateRefuted}[vulnerable]
			if got.State != want {
				t.Fatalf("state %s, want %s: %s", got.State, want, got.Validation.Note)
			}
			if sawScript.Load() != 0 {
				t.Fatal("the stored-XSS probe must never send a script tag")
			}
			mu.Lock()
			left := bio
			mu.Unlock()
			if strings.Contains(left, "assay-") {
				t.Fatalf("probe value left behind after the check: %q", left)
			}
			w.assertSameHostOnly(t)
		})
	}
}

func TestXSSStoredNeedsAParameter(t *testing.T) {
	w := newWebLab(t, http.NotFoundHandler())
	if got := w.check(t, extra("XS", model.ClassXSSStored, "POST", "/profile", "")); got.State != model.StateTheorized {
		t.Fatalf("state %s", got.State)
	}
}

// ---- registration ----------------------------------------------------------

func TestNewCheckersAreRegisteredAndOnlyOtherHasNone(t *testing.T) {
	for _, c := range []model.Class{model.ClassAuthBypass, model.ClassJWTWeak, model.ClassMassAssignment,
		model.ClassXSSStored, model.ClassRateLimit, model.ClassCSRF, model.ClassSSRF} {
		if Lookup(c) == nil {
			t.Errorf("no checker registered for %s", c)
		}
	}
	for _, c := range model.AllClasses {
		if c == model.ClassOther {
			if Lookup(c) != nil {
				t.Error("class other must stay without a checker")
			}
			continue
		}
		if Lookup(c) == nil {
			t.Errorf("class %s has no checker", c)
		}
	}
}

func TestAuthBypassTrailingSlashStillValidatesWhenSiblingsAre404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/admin", func(rw http.ResponseWriter, r *http.Request) {
		http.Error(rw, "forbidden", http.StatusForbidden)
	})
	mux.HandleFunc("/admin/", func(rw http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin/" {
			io.WriteString(rw, "admin console behind the trailing slash")
			return
		}
		http.NotFound(rw, r)
	})
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) { http.NotFound(rw, r) })
	w := newWebLab(t, mux)
	got := w.check(t, extra("AB", model.ClassAuthBypass, "GET", "/admin", ""))
	if got.State != model.StateValidated || !strings.Contains(got.Validation.Note, "trailing-slash") {
		t.Fatalf("state %s: %s", got.State, got.Validation.Note)
	}
}
