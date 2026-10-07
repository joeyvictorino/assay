package validate

// These tests run the checkers against small purpose-built httptest handlers
// that mimic each vulnerable behavior of labs/synthetic-ops (and a hardened
// variant of each). The root module cannot import the lab module (it is a
// separate Go module so its SQLite dependency stays out of the harness), so
// the end-to-end run against the real lab binary lives in a CI step owned by
// S4, not here.

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/labs"
	"github.com/joeyvictorino/assay/internal/model"
)

const (
	secretMarker = "MIMIC-SECRET-0001"
	readmeMarker = "MIMIC-README-0001"
)

// recordingGate allows one host and remembers every URL it was asked about.
type recordingGate struct {
	host string
	mu   sync.Mutex
	urls []string
}

func (g *recordingGate) Allow(_ context.Context, target string) model.Decision {
	g.mu.Lock()
	g.urls = append(g.urls, target)
	g.mu.Unlock()
	u, err := url.Parse(target)
	if err != nil || u.Host != g.host {
		return model.Decision{Effect: model.EffectDeny, Reason: "OUT_OF_SCOPE", Rationale: "host"}
	}
	return model.Decision{Effect: model.EffectAllow, Reason: "IN_SCOPE"}
}

// mimic is a lab whose handlers imitate synthetic-ops. secure flips every
// endpoint to its hardened behavior so refutation can be tested.
type mimic struct {
	secure   bool
	flaky    bool // /config alternates marker presence
	srv      *httptest.Server
	client   *httpx.Client
	gate     *recordingGate
	aud      *httpx.MemAuditor
	mu       sync.Mutex
	sessions map[string]string // token -> username
	hits     int
}

var mimicUsers = map[string]struct {
	id, pass, email string
}{
	"admin": {"1", "admin123", "admin@mimic.lab"},
	"alice": {"2", "Wonderland-1", "alice@mimic.lab"},
	"bob":   {"3", "Builder-22", "bob@mimic.lab"},
}

func newMimic(t *testing.T, secure bool) *mimic {
	t.Helper()
	m := &mimic{secure: secure, sessions: map[string]string{}}
	m.srv = httptest.NewServer(m.handler())
	t.Cleanup(m.srv.Close)
	u, _ := url.Parse(m.srv.URL)
	m.gate = &recordingGate{host: u.Host}
	m.aud = &httpx.MemAuditor{}
	m.client = &httpx.Client{Gate: m.gate, Auditor: m.aud, RunID: "t", Timeout: 5 * time.Second}
	return m
}

func (m *mimic) Name() string                 { return "mimic" }
func (m *mimic) BaseURL() string              { return m.srv.URL }
func (m *mimic) Health(context.Context) error { return nil }
func (m *mimic) Setup(context.Context) error  { return nil }
func (m *mimic) Accounts() []labs.Account {
	return []labs.Account{
		{Role: "admin", Username: "admin", Password: "admin123"},
		{Role: "user", Username: "alice", Password: "Wonderland-1"},
		{Role: "user", Username: "bob", Password: "Builder-22"},
	}
}
func (m *mimic) GroundTruth() ([]labs.GroundTruthEntry, bool) {
	return []labs.GroundTruthEntry{
		{ID: "M-ADMIN", Lab: "mimic", Class: model.ClassAuthMissing, Method: "GET", PathTemplate: "/admin", Severity: model.SevHigh, AuthRequired: true},
		{ID: "M-OPEN", Lab: "mimic", Class: model.ClassAuthMissing, Method: "GET", PathTemplate: "/public", Severity: model.SevInfo, AuthRequired: false},
		{ID: "M-CFG", Lab: "mimic", Class: model.ClassInfoDisclosure, Method: "GET", PathTemplate: "/config", Severity: model.SevHigh, Marker: secretMarker},
	}, true
}

func (m *mimic) Login(ctx context.Context, acct labs.Account) (map[string]string, string, error) {
	body, _ := json.Marshal(map[string]string{"username": acct.Username, "password": acct.Password})
	status, _, resp, err := m.client.Do(ctx, "POST", m.srv.URL+"/login", map[string]string{"Content-Type": "application/json"}, body)
	if err != nil {
		return nil, "", err
	}
	if status == 401 {
		return nil, "", fmt.Errorf("login failed: %d: %w", status, labs.ErrLoginRejected)
	}
	if status != 200 {
		return nil, "", fmt.Errorf("login failed: %d", status)
	}
	var out struct {
		Token string `json:"token"`
		ID    string `json:"id"`
	}
	_ = json.Unmarshal(resp, &out)
	return map[string]string{"Authorization": "Bearer " + out.Token}, out.ID, nil
}

func (m *mimic) handler() http.Handler {
	mux := http.NewServeMux()
	wrap := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if m.secure {
				w.Header().Set("Content-Security-Policy", "default-src 'self'")
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.Header().Set("X-Frame-Options", "DENY")
				if o := r.Header.Get("Origin"); o == "https://trusted.example" {
					w.Header().Set("Access-Control-Allow-Origin", o)
				}
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /{$}", wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>home</html>"))
	}))
	mux.HandleFunc("GET /boom", wrap(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		if m.secure {
			w.Write([]byte("internal error"))
			return
		}
		w.Write([]byte("<pre>panic: nil map\n\ngoroutine 1 [running]:\nmain.boom()\n\t/src/handlers/debug.go:31</pre>"))
	}))
	mux.HandleFunc("GET /config", wrap(func(w http.ResponseWriter, r *http.Request) {
		m.mu.Lock()
		m.hits++
		n := m.hits
		m.mu.Unlock()
		if m.secure || (m.flaky && n%2 == 0) {
			http.Error(w, "not found", 404)
			return
		}
		w.Write([]byte(`{"admin_api_key":"` + secretMarker + `"}`))
	}))
	mux.HandleFunc("GET /admin", wrap(func(w http.ResponseWriter, r *http.Request) {
		if m.secure {
			http.Error(w, "unauthorized", 401)
			return
		}
		w.Write([]byte(`{"users":3}`))
	}))
	mux.HandleFunc("GET /public", wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	}))
	mux.HandleFunc("GET /redirect", wrap(func(w http.ResponseWriter, r *http.Request) {
		to := r.URL.Query().Get("to")
		if m.secure || !strings.HasPrefix(to, "http") {
			to = "/"
		}
		http.Redirect(w, r, to, 302)
	}))
	mux.HandleFunc("GET /search", wrap(func(w http.ResponseWriter, r *http.Request) {
		term := r.URL.Query().Get("term")
		if m.secure {
			term = html.EscapeString(term)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<html><p>Results for: %s</p></html>", term)
	}))
	mux.HandleFunc("GET /api/search", wrap(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		// Emulates: SELECT ... WHERE name LIKE '%<q>%'
		n := 0
		if !m.secure && strings.HasSuffix(q, "' OR 'a%'='a") {
			n = 5
		}
		out := make([]map[string]any, n)
		for i := range out {
			out[i] = map[string]any{"id": i + 1, "name": fmt.Sprintf("Item %d", i+1)}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}))
	mux.HandleFunc("GET /files", wrap(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if m.secure || !strings.HasPrefix(name, "../") || !strings.HasSuffix(name, "README.md") {
			http.Error(w, "not found", 404)
			return
		}
		if strings.Count(name, "../") != 1 {
			http.Error(w, "not found", 404)
			return
		}
		w.Write([]byte("# mimic lab\nmarker: " + readmeMarker + "\n"))
	}))
	mux.HandleFunc("POST /login", wrap(func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		json.NewDecoder(r.Body).Decode(&in)
		u, ok := mimicUsers[in["username"]]
		if !ok || u.pass != in["password"] || (m.secure && in["username"] == "admin") {
			http.Error(w, "nope", 401)
			return
		}
		tok := "tok-" + in["username"]
		m.mu.Lock()
		m.sessions[tok] = in["username"]
		m.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]string{"token": tok, "id": u.id})
	}))
	mux.HandleFunc("GET /users/{id}", wrap(func(w http.ResponseWriter, r *http.Request) {
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		m.mu.Lock()
		caller, ok := m.sessions[tok]
		m.mu.Unlock()
		if !ok {
			http.Error(w, "unauthorized", 401)
			return
		}
		id := r.PathValue("id")
		for name, u := range mimicUsers {
			if u.id == id {
				if m.secure && name != caller {
					http.Error(w, "forbidden", 403)
					return
				}
				json.NewEncoder(w).Encode(map[string]any{"id": id, "username": name, "email": u.email})
				return
			}
		}
		http.Error(w, "not found", 404)
	}))
	return mux
}

func (m *mimic) env() Env {
	return Env{HTTP: m.client, Lab: m, Marker: secretMarker,
		Markers: map[string]string{"traversal-file": "README.md", "traversal-marker": readmeMarker},
		Now:     func() time.Time { return time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC) }}
}

func finding(id string, class model.Class, method, path, param string) model.Finding {
	return model.Finding{ID: id, Lab: "mimic", Class: class, State: model.StateTheorized,
		Location: model.Location{Method: method, PathTemplate: path, Param: param}, Severity: model.SevMedium}
}

var allFindings = []model.Finding{
	finding("F-HDR", model.ClassSecurityHeaders, "GET", "/", ""),
	finding("F-ERR", model.ClassVerboseError, "GET", "/boom", ""),
	finding("F-CORS", model.ClassCORS, "GET", "/", ""),
	finding("F-INFO", model.ClassInfoDisclosure, "GET", "/config", ""),
	finding("F-FILE", model.ClassSensitiveFile, "GET", "/config", ""),
	finding("F-AUTH", model.ClassAuthMissing, "GET", "/admin", ""),
	finding("F-REDIR", model.ClassOpenRedirect, "GET", "/redirect", "to"),
	finding("F-XSS", model.ClassXSSReflected, "GET", "/search", "term"),
	finding("F-SQLI", model.ClassSQLi, "GET", "/api/search", "q"),
	finding("F-TRAV", model.ClassPathTraversal, "GET", "/files", "name"),
	finding("F-IDOR", model.ClassIDOR, "GET", "/users/{id}", "id"),
	finding("F-CREDS", model.ClassDefaultCreds, "POST", "/login", "password"),
}

func TestAllCheckersValidateVulnerableMimic(t *testing.T) {
	m := newMimic(t, false)
	out := Validate(context.Background(), allFindings, m.env())
	if len(out) != len(allFindings) {
		t.Fatalf("len %d", len(out))
	}
	for i, f := range out {
		if f.ID != allFindings[i].ID {
			t.Fatalf("order changed at %d", i)
		}
		if f.State != model.StateValidated {
			t.Errorf("%s (%s): state %s note %q", f.ID, f.Class, f.State, f.Validation.Note)
			continue
		}
		if f.Validation == nil || len(f.Validation.Evidence) < MinObservations || f.Validation.Checker != string(f.Class) {
			t.Errorf("%s: validation %+v", f.ID, f.Validation)
		}
		for _, ev := range f.Validation.Evidence {
			if strings.Contains(ev.Marker, "admin123") || strings.Contains(ev.Marker, "Wonderland") {
				t.Errorf("%s: evidence leaks a password: %+v", f.ID, ev)
			}
		}
	}
	// Inputs are not mutated.
	for _, f := range allFindings {
		if f.State != model.StateTheorized || f.Validation != nil {
			t.Fatal("input findings mutated")
		}
	}
	// The open-redirect target host must never have been requested (it only
	// ever appears inside a query parameter sent to the lab).
	for _, raw := range m.gate.urls {
		u, err := url.Parse(raw)
		if err != nil || strings.HasSuffix(u.Hostname(), ".invalid") {
			t.Fatalf("redirect target was requested: %s", raw)
		}
	}
	// The XSS payload never carries a script.
	for _, u := range m.gate.urls {
		if strings.Contains(strings.ToLower(u), "script") {
			t.Fatalf("script payload observed: %s", u)
		}
	}
	// Every exchange left an audit record.
	if len(m.aud.ByKind("tool_call")) < 24 {
		t.Fatalf("audit records: %d", len(m.aud.ByKind("tool_call")))
	}
}

func TestAllCheckersRefuteHardenedMimic(t *testing.T) {
	m := newMimic(t, true)
	out := Validate(context.Background(), allFindings, m.env())
	for _, f := range out {
		if f.State != model.StateRefuted {
			t.Errorf("%s (%s): state %s note %q", f.ID, f.Class, f.State, f.Validation.Note)
		}
		if f.State == model.StateValidated {
			t.Fatalf("%s: hardened endpoint validated", f.ID)
		}
	}
}

func TestNoCheckerStaysTheorized(t *testing.T) {
	m := newMimic(t, false)
	out := Validate(context.Background(), []model.Finding{finding("F-CSRF", model.ClassCSRF, "POST", "/x", "")}, m.env())
	if out[0].State != model.StateTheorized || out[0].Validation.Checker != "none" {
		t.Fatalf("%+v", out[0].Validation)
	}
}

func TestDeclinedUntouched(t *testing.T) {
	m := newMimic(t, false)
	f := finding("F-D", model.ClassSQLi, "GET", "/api/search", "q")
	f.State = model.StateDeclined
	out := Validate(context.Background(), []model.Finding{f}, m.env())
	if out[0].State != model.StateDeclined || out[0].Validation != nil {
		t.Fatal("declined findings must not be checked")
	}
}

func TestGateDenialIsTheorizedNotValidated(t *testing.T) {
	m := newMimic(t, false)
	// Both the checker client and the lab adapter's client are denied.
	m.client = &httpx.Client{Gate: &recordingGate{host: "nobody.invalid"}, Auditor: &httpx.MemAuditor{}}
	env := m.env()
	out := Validate(context.Background(), allFindings, env)
	for _, f := range out {
		if f.State == model.StateValidated || f.State == model.StateRefuted {
			t.Errorf("%s: %s after gate denial", f.ID, f.State)
		}
	}
}

func TestIncompleteEnvIsTheorized(t *testing.T) {
	out := Validate(context.Background(), allFindings[:1], Env{})
	if out[0].State != model.StateTheorized || !strings.Contains(out[0].Validation.Note, "incomplete") {
		t.Fatalf("%+v", out[0].Validation)
	}
}

func TestFlakyMarkerNeedsTwoObservations(t *testing.T) {
	m := newMimic(t, false)
	m.flaky = true
	out := Validate(context.Background(), []model.Finding{finding("F-INFO", model.ClassInfoDisclosure, "GET", "/config", "")}, m.env())
	if out[0].State != model.StateTheorized {
		t.Fatalf("one observation must not validate: %s %q", out[0].State, out[0].Validation.Note)
	}
}

func TestAuthMissingRequiresGroundTruthFlag(t *testing.T) {
	m := newMimic(t, false)
	out := Validate(context.Background(), []model.Finding{finding("F-PUB", model.ClassAuthMissing, "GET", "/public", "")}, m.env())
	if out[0].State != model.StateTheorized {
		t.Fatalf("path not marked auth-required must stay theorized: %s", out[0].State)
	}
}

func TestParamlessInjectionFindingsStayTheorized(t *testing.T) {
	m := newMimic(t, false)
	fs := []model.Finding{
		finding("F-X", model.ClassXSSReflected, "GET", "/search", ""),
		finding("F-S", model.ClassSQLi, "GET", "/api/search", ""),
	}
	for _, f := range Validate(context.Background(), fs, m.env()) {
		if f.State != model.StateTheorized {
			t.Errorf("%s: %s", f.ID, f.State)
		}
	}
}

func TestMinObservationsEnforced(t *testing.T) {
	// A checker result claiming "validated" with one observation is downgraded.
	res := model.ValidationResult{State: model.StateValidated, Evidence: []model.Evidence{{Kind: "x"}}}
	m := newMimic(t, false)
	env := m.env()
	got := enforce(res, env)
	if got.State != model.StateTheorized || !strings.Contains(got.Note, "1 observation") {
		t.Fatalf("%+v", got)
	}
}

func TestCheckersCoverListedClasses(t *testing.T) {
	want := []model.Class{model.ClassSecurityHeaders, model.ClassVerboseError, model.ClassCORS, model.ClassOpenRedirect,
		model.ClassXSSReflected, model.ClassInfoDisclosure, model.ClassSensitiveFile, model.ClassAuthMissing, model.ClassIDOR,
		model.ClassDefaultCreds, model.ClassSQLi, model.ClassPathTraversal}
	for _, c := range want {
		if Lookup(c) == nil {
			t.Errorf("no checker for %s", c)
		}
	}
	if Lookup(model.ClassCSRF) != nil {
		t.Error("unexpected csrf checker")
	}
	cs := Checkers()
	for i := 1; i < len(cs); i++ {
		if indexOf(cs[i-1].Class()) >= indexOf(cs[i].Class()) {
			t.Fatal("checkers must follow model.AllClasses order")
		}
	}
}

func indexOf(c model.Class) int {
	for i, x := range model.AllClasses {
		if x == c {
			return i
		}
	}
	return -1
}

func TestTargetURLAndQuery(t *testing.T) {
	m := newMimic(t, false)
	env := m.env()
	f := finding("x", model.ClassIDOR, "GET", "/users/{id}/orders/{order}", "")
	got := targetURL(env, f, map[string]string{"id": "4 2"})
	if got != m.srv.URL+"/users/4%202/orders/1" {
		t.Fatalf("got %s", got)
	}
	f.Location.PathTemplate = ""
	if targetURL(env, f, nil) != m.srv.URL+"/" {
		t.Fatal("empty template")
	}
	if q := withQuery(m.srv.URL+"/s?a=1", "q", "x y&z"); !strings.Contains(q, "a=1") || !strings.Contains(q, "q=x+y%26z") {
		t.Fatalf("query %s", q)
	}
	if mk := newMarker(); !strings.HasPrefix(mk, "assay-") || len(mk) != 18 {
		t.Fatalf("marker %s", mk)
	}
}

func TestResultCount(t *testing.T) {
	cases := []struct {
		body string
		ct   string
		want int
	}{
		{`[1,2,3]`, "application/json", 3},
		{`{"total":3,"results":[1,2]}`, "application/json", 2},
		{`{"a":1}`, "application/json", -1},
		{`<table><tr></tr><tr></tr></table>`, "text/html", 2},
		{`<ul><li>a</li></ul>`, "text/html", 1},
		{`<p>none</p>`, "text/html", 0},
		{`plain`, "text/plain", -1},
		{`[broken`, "application/json", -1},
	}
	for _, tc := range cases {
		r := &httpx.Response{Body: []byte(tc.body), Headers: map[string]string{"Content-Type": tc.ct}}
		if got := resultCount(r); got != tc.want {
			t.Errorf("%q: got %d want %d", tc.body, got, tc.want)
		}
	}
}

func TestPickTwo(t *testing.T) {
	a, b, ok := pickTwo([]labs.Account{{Role: "admin", Username: "root"}, {Role: "user", Username: "u1"}, {Role: "user", Username: "u2"}})
	if !ok || a.Username != "u1" || b.Username != "u2" {
		t.Fatalf("%v %v %v", a, b, ok)
	}
	a, b, ok = pickTwo([]labs.Account{{Role: "admin", Username: "root"}, {Role: "user", Username: "u1"}})
	if !ok || a.Username != "root" || b.Username != "u1" {
		t.Fatalf("%v %v %v", a, b, ok)
	}
	if _, _, ok = pickTwo([]labs.Account{{Username: "only"}, {Username: "only"}}); ok {
		t.Fatal("duplicates must not count")
	}
}

func TestSQLiPayloadsAreNonDestructive(t *testing.T) {
	for _, p := range sqliPairs {
		for _, s := range []string{p.truthy, p.falsy} {
			up := strings.ToUpper(s)
			for _, bad := range []string{"DROP", "DELETE", "UPDATE", "INSERT", ";", "SLEEP", "WAITFOR", "BENCHMARK", "PG_SLEEP", "--", "/*"} {
				if strings.Contains(up, bad) {
					t.Errorf("payload %q contains %q", s, bad)
				}
			}
		}
	}
}
