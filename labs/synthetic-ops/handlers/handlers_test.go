package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// LEAD: this list mirrors model.AllClasses in the root module. The lab
// module cannot import the root module, so the strings are duplicated here
// and the root-side test in internal/labs/syntheticops checks the file
// against model.AllClasses as well.
var knownClasses = map[string]bool{
	"sqli": true, "xss-reflected": true, "xss-stored": true, "idor": true, "auth-missing": true,
	"auth-bypass": true, "default-creds": true, "open-redirect": true, "path-traversal": true,
	"ssrf": true, "info-disclosure": true, "security-headers": true, "cors-misconfig": true,
	"mass-assignment": true, "jwt-weak": true, "rate-limit-missing": true, "verbose-error": true,
	"sensitive-file": true, "csrf": true, "other": true,
}

func newTestServer(t *testing.T, opts Options) *httptest.Server {
	t.Helper()
	db, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := Seed(db); err != nil {
		t.Fatal(err)
	}
	if opts.PublicDir == "" {
		opts.PublicDir = "../public"
	}
	srv := httptest.NewServer(New(db, opts))
	t.Cleanup(func() { srv.Close(); db.Close() })
	return srv
}

func get(t *testing.T, srv *httptest.Server, path string, headers map[string]string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func postJSON(t *testing.T, srv *httptest.Server, path string, body any, headers map[string]string) (*http.Response, string) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, srv.URL+path, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	return resp, string(rb)
}

func login(t *testing.T, srv *httptest.Server, user, pass string) (token string, id int) {
	t.Helper()
	resp, body := postJSON(t, srv, "/api/login", map[string]string{"username": user, "password": pass}, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("login %s: %d %s", user, resp.StatusCode, body)
	}
	var out struct {
		Token  string `json:"token"`
		UserID int    `json:"user_id"`
	}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	return out.Token, out.UserID
}

func TestDefaultCredentials(t *testing.T) {
	srv := newTestServer(t, Options{})
	tok, id := login(t, srv, "admin", "admin123")
	if tok == "" || id != 1 {
		t.Fatalf("token=%q id=%d", tok, id)
	}
	resp, _ := postJSON(t, srv, "/api/login", map[string]string{"username": "admin", "password": "wrong"}, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("wrong password accepted: %d", resp.StatusCode)
	}
}

func TestIDOR(t *testing.T) {
	srv := newTestServer(t, Options{})
	tok, id := login(t, srv, "alice", "Wonderland-1")
	resp, body := get(t, srv, "/api/users/"+strconv.Itoa(id+1), map[string]string{"Authorization": "Bearer " + tok})
	if resp.StatusCode != 200 || !strings.Contains(body, `"username":"bob"`) {
		t.Fatalf("IDOR not reproduced: %d %s", resp.StatusCode, body)
	}
	if strings.Contains(body, "Builder-22") {
		t.Fatal("password must not be in the record")
	}
	resp, _ = get(t, srv, "/api/users/2", nil)
	if resp.StatusCode != 401 {
		t.Fatalf("unauthenticated read must be refused: %d", resp.StatusCode)
	}
}

func TestSQLiBooleanDifferential(t *testing.T) {
	srv := newTestServer(t, Options{})
	count := func(q string) int {
		resp, body := get(t, srv, "/api/search?q="+url.QueryEscape(q), nil)
		if resp.StatusCode != 200 {
			t.Fatalf("%q: %d %s", q, resp.StatusCode, body)
		}
		var arr []any
		if err := json.Unmarshal([]byte(body), &arr); err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		return len(arr)
	}
	base := count("zzz-no-such-product")
	// LIKE '%...%' context: the payload closes the literal with a wildcard.
	tautology := count("zzz%' OR '1%'='1")
	contradiction := count("zzz%' AND '1%'='2")
	if base != 0 || contradiction != 0 || tautology != 5 {
		t.Fatalf("base=%d tautology=%d contradiction=%d", base, tautology, contradiction)
	}
}

func TestStoredXSS(t *testing.T) {
	srv := newTestServer(t, Options{})
	tok, id := login(t, srv, "bob", "Builder-22")
	payload := "<b>assay-stored-marker</b>"
	resp, body := postJSON(t, srv, "/api/profile", map[string]string{"bio": payload}, map[string]string{"Authorization": "Bearer " + tok})
	if resp.StatusCode != 200 {
		t.Fatalf("profile update: %d %s", resp.StatusCode, body)
	}
	resp, body = get(t, srv, "/profile/"+strconv.Itoa(id), nil)
	if resp.StatusCode != 200 || !strings.Contains(body, payload) || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("stored XSS not reproduced: %d %s", resp.StatusCode, body)
	}
	resp, _ = postJSON(t, srv, "/api/profile", map[string]string{"bio": "x"}, nil)
	if resp.StatusCode != 401 {
		t.Fatalf("unauthenticated profile write must be refused: %d", resp.StatusCode)
	}
}

func TestReflectedXSS(t *testing.T) {
	srv := newTestServer(t, Options{})
	payload := "<b>assay-reflected-marker</b>"
	resp, body := get(t, srv, "/search?term="+url.QueryEscape(payload), nil)
	if resp.StatusCode != 200 || !strings.Contains(body, payload) {
		t.Fatalf("reflected XSS not reproduced: %d %s", resp.StatusCode, body)
	}
}

func TestOpenRedirect(t *testing.T) {
	srv := newTestServer(t, Options{})
	target := "http://assay-marker.invalid/landing"
	resp, _ := get(t, srv, "/redirect?to="+url.QueryEscape(target), nil)
	if resp.StatusCode != 302 || resp.Header.Get("Location") != target {
		t.Fatalf("open redirect not reproduced: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestPathTraversal(t *testing.T) {
	srv := newTestServer(t, Options{})
	resp, body := get(t, srv, "/files?name=hello.txt", nil)
	if resp.StatusCode != 200 || !strings.Contains(body, "hello from synthetic-ops") {
		t.Fatalf("benign file: %d %s", resp.StatusCode, body)
	}
	resp, body = get(t, srv, "/files?name="+url.QueryEscape("../README.md"), nil)
	if resp.StatusCode != 200 || !strings.Contains(body, "SYNTH-LAB-README-MARKER-0001") {
		t.Fatalf("traversal not reproduced: %d %s", resp.StatusCode, body[:min(len(body), 200)])
	}
}

func TestAdminStatsMissingAuth(t *testing.T) {
	srv := newTestServer(t, Options{})
	resp, body := get(t, srv, "/api/admin/stats", nil)
	if resp.StatusCode != 200 || !strings.Contains(body, `"users":3`) {
		t.Fatalf("missing auth not reproduced: %d %s", resp.StatusCode, body)
	}
}

func TestSSRFDefaultRefusesNonLoopback(t *testing.T) {
	srv := newTestServer(t, Options{})
	resp, body := get(t, srv, "/api/fetch?url="+url.QueryEscape("http://example.com/"), nil)
	if resp.StatusCode != 403 || !strings.Contains(body, "refused") {
		t.Fatalf("non-loopback must be refused by default: %d %s", resp.StatusCode, body)
	}
	resp, body = get(t, srv, "/api/fetch?url="+url.QueryEscape("http://93.184.216.34/"), nil)
	if resp.StatusCode != 403 {
		t.Fatalf("non-loopback IP must be refused by default: %d %s", resp.StatusCode, body)
	}
	// Loopback is reachable: the service fetches itself.
	resp, body = get(t, srv, "/api/fetch?url="+url.QueryEscape(srv.URL+"/debug/config"), nil)
	if resp.StatusCode != 200 || !strings.Contains(body, SecretMarker) {
		t.Fatalf("loopback SSRF not reproduced: %d %s", resp.StatusCode, body)
	}
	resp, _ = get(t, srv, "/api/fetch?url="+url.QueryEscape("file:///etc/hosts"), nil)
	if resp.StatusCode != 400 {
		t.Fatalf("non-http scheme: %d", resp.StatusCode)
	}
}

func TestSSRFUnsafeFlagEnablesNonLoopbackCheck(t *testing.T) {
	// With the flag on, the loopback rail is gone; we verify the code path by
	// using an unroutable host that fails fast rather than a real external host.
	srv := newTestServer(t, Options{UnsafeSSRF: true})
	resp, _ := get(t, srv, "/api/fetch?url="+url.QueryEscape("http://192.0.2.1:9/"), nil)
	if resp.StatusCode == 403 {
		t.Fatal("unsafe mode must not apply the loopback rail")
	}
}

func TestDebugConfigAndBackup(t *testing.T) {
	srv := newTestServer(t, Options{})
	resp, body := get(t, srv, "/debug/config", nil)
	if resp.StatusCode != 200 || !strings.Contains(body, SecretMarker) {
		t.Fatalf("info disclosure not reproduced: %d %s", resp.StatusCode, body)
	}
	resp, body = get(t, srv, "/backup.sql", nil)
	if resp.StatusCode != 200 || !strings.Contains(body, BackupMarker) {
		t.Fatalf("sensitive file not reproduced: %d %s", resp.StatusCode, body)
	}
}

func TestVerboseError(t *testing.T) {
	srv := newTestServer(t, Options{})
	resp, body := get(t, srv, "/api/boom", nil)
	if resp.StatusCode != 500 || !strings.Contains(body, "goroutine") || !strings.Contains(body, "panic:") {
		t.Fatalf("verbose error not reproduced: %d %s", resp.StatusCode, body)
	}
}

func TestMissingSecurityHeadersAndCORS(t *testing.T) {
	srv := newTestServer(t, Options{})
	resp, _ := get(t, srv, "/", nil)
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy", "Strict-Transport-Security"} {
		if resp.Header.Get(h) != "" {
			t.Fatalf("%s must be absent", h)
		}
	}
	if resp.Header.Get("Access-Control-Allow-Origin") != "*" || resp.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("CORS misconfiguration not reproduced: %v", resp.Header)
	}
	resp, body := get(t, srv, "/healthz", nil)
	if resp.StatusCode != 200 || !strings.Contains(body, "ok") {
		t.Fatalf("healthz: %d %s", resp.StatusCode, body)
	}
}

type gtEntry struct {
	ID           string `json:"id"`
	Lab          string `json:"lab"`
	Class        string `json:"class"`
	Method       string `json:"method"`
	PathTemplate string `json:"path_template"`
	Param        string `json:"param"`
	Severity     string `json:"severity"`
	CWE          string `json:"cwe"`
	SourceRef    string `json:"source_ref"`
	Description  string `json:"description"`
}

// TestGroundTruthSelfConsistent checks every ground-truth entry points at a
// line in this package that carries a VULN marker for the same class.
func TestGroundTruthSelfConsistent(t *testing.T) {
	data, err := os.ReadFile("../ground_truth.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []gtEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) < 12 {
		t.Fatalf("only %d entries", len(entries))
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.ID] {
			t.Errorf("%s: duplicate id", e.ID)
		}
		seen[e.ID] = true
		if e.Lab != "synthetic-ops" {
			t.Errorf("%s: lab %q", e.ID, e.Lab)
		}
		if !knownClasses[e.Class] {
			t.Errorf("%s: unknown class %q", e.ID, e.Class)
		}
		switch e.Severity {
		case "info", "low", "medium", "high", "critical":
		default:
			t.Errorf("%s: severity %q", e.ID, e.Severity)
		}
		if !strings.HasPrefix(e.CWE, "CWE-") || !strings.HasPrefix(e.PathTemplate, "/") || e.Method == "" || e.Description == "" {
			t.Errorf("%s: malformed entry %+v", e.ID, e)
		}
		if strings.Contains(e.PathTemplate, "/1") || strings.Contains(e.PathTemplate, "/2") {
			t.Errorf("%s: path template %q must use {id}, not a literal id", e.ID, e.PathTemplate)
		}
		file, lineStr, ok := strings.Cut(e.SourceRef, ":")
		if !ok || !strings.HasPrefix(file, "handlers/") {
			t.Errorf("%s: source_ref %q", e.ID, e.SourceRef)
			continue
		}
		line, err := strconv.Atoi(lineStr)
		if err != nil {
			t.Errorf("%s: source_ref line %q", e.ID, lineStr)
			continue
		}
		src, err := os.ReadFile(filepath.Join("..", file))
		if err != nil {
			t.Errorf("%s: %v", e.ID, err)
			continue
		}
		lines := strings.Split(string(src), "\n")
		if line < 1 || line > len(lines) {
			t.Errorf("%s: line %d out of range", e.ID, line)
			continue
		}
		want := fmt.Sprintf("// VULN: %s", e.Class)
		if !strings.Contains(lines[line-1], want) {
			t.Errorf("%s: %s does not carry %q: %s", e.ID, e.SourceRef, want, strings.TrimSpace(lines[line-1]))
		}
	}
}
