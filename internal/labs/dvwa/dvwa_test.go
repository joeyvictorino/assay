package dvwa

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/joeyvictorino/assay/internal/httpx"
	"github.com/joeyvictorino/assay/internal/labs"
	"github.com/joeyvictorino/assay/internal/model"
)

type hostGate struct{ host string }

func (g hostGate) Allow(_ context.Context, target string) model.Decision {
	u, err := url.Parse(target)
	if err != nil || u.Host != g.host {
		return model.Decision{Effect: model.EffectDeny, Reason: "OUT_OF_SCOPE"}
	}
	return model.Decision{Effect: model.EffectAllow, Reason: "IN_SCOPE"}
}

// fakeDVWA mimics the setup/login flow: CSRF token in forms, PHPSESSID
// cookie, 302 to index.php on success and back to login.php on failure.
type fakeDVWA struct {
	dbCreated bool
	loggedIn  map[string]bool
	sawLow    bool
}

func (f *fakeDVWA) handler() http.Handler {
	mux := http.NewServeMux()
	page := func(w http.ResponseWriter, title string) {
		http.SetCookie(w, &http.Cookie{Name: "PHPSESSID", Value: "sess123", Path: "/"})
		w.Write([]byte(`<html><title>` + title + `</title><form><input type='hidden' name='user_token' value='deadbeef01' /></form></html>`))
	}
	mux.HandleFunc("GET /login.php", func(w http.ResponseWriter, r *http.Request) { page(w, "Login :: DVWA") })
	mux.HandleFunc("GET /setup.php", func(w http.ResponseWriter, r *http.Request) { page(w, "Setup :: DVWA") })
	mux.HandleFunc("POST /setup.php", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("user_token") != "deadbeef01" || r.Form.Get("create_db") == "" {
			w.WriteHeader(403)
			return
		}
		f.dbCreated = true
		http.Redirect(w, r, "/setup.php", http.StatusFound)
	})
	mux.HandleFunc("POST /login.php", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		c, _ := r.Cookie("security")
		if c != nil && c.Value == "low" {
			f.sawLow = true
		}
		if r.Form.Get("user_token") != "deadbeef01" || !f.dbCreated {
			http.Redirect(w, r, "/login.php", http.StatusFound)
			return
		}
		u, p := r.Form.Get("username"), r.Form.Get("password")
		if (u == "admin" && p == "password") || (u == "gordonb" && p == "abc123") {
			f.loggedIn[u] = true
			http.SetCookie(w, &http.Cookie{Name: "PHPSESSID", Value: "sess-" + u, Path: "/"})
			http.Redirect(w, r, "index.php", http.StatusFound)
			return
		}
		http.Redirect(w, r, "login.php", http.StatusFound)
	})
	return mux
}

func newLab(t *testing.T) (*Lab, *fakeDVWA) {
	t.Helper()
	f := &fakeDVWA{loggedIn: map[string]bool{}}
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	client := &httpx.Client{Gate: hostGate{u.Host}, Auditor: &httpx.MemAuditor{}}
	return New(srv.URL+"/", "../../../labs/dvwa/reference_findings.json", client), f
}

func TestSetupAndLogin(t *testing.T) {
	lab, f := newLab(t)
	ctx := context.Background()
	if lab.Name() != Name {
		t.Fatal("name")
	}
	if err := lab.Health(ctx); err != nil {
		t.Fatal(err)
	}
	if lab.Cookie() != "" {
		t.Fatal("cookie before setup must be empty")
	}
	if err := lab.Setup(ctx); err != nil {
		t.Fatal(err)
	}
	if !f.dbCreated || !f.loggedIn["admin"] || !f.sawLow {
		t.Fatalf("setup flow incomplete: %+v", f)
	}
	if c := lab.Cookie(); !strings.Contains(c, "PHPSESSID=sess-admin") || !strings.Contains(c, "security=low") {
		t.Fatalf("cookie %q", c)
	}
	hdrs, id, err := lab.Login(ctx, labs.Account{Username: "gordonb", Password: "abc123"})
	if err != nil || id != "gordonb" || !strings.Contains(hdrs["Cookie"], "sess-gordonb") {
		t.Fatalf("login: %v %s %v", hdrs, id, err)
	}
	if _, _, err := lab.Login(ctx, labs.Account{Username: "pablo", Password: "wrong"}); err == nil {
		t.Fatal("bad login must fail")
	}
	accts := lab.Accounts()
	if len(accts) != 5 || accts[0].Role != "admin" || accts[0].Password != "password" {
		t.Fatalf("accounts %+v", accts)
	}
	refs, exhaustive := lab.GroundTruth()
	if exhaustive || len(refs) < 8 || len(refs) > 10 {
		t.Fatalf("refs exhaustive=%v n=%d", exhaustive, len(refs))
	}
}

func TestLoginBeforeSetupFails(t *testing.T) {
	lab, _ := newLab(t)
	if _, _, err := lab.Login(context.Background(), labs.Account{Username: "admin", Password: "password"}); err == nil {
		t.Fatal("login without database must fail")
	}
}

func TestTokenParsing(t *testing.T) {
	if tokenFrom([]byte(`<input value='abc123' name='user_token'>`)) != "abc123" {
		t.Fatal("reversed attribute order")
	}
	if tokenFrom([]byte(`<input name="user_token" value="ff00">`)) != "ff00" {
		t.Fatal("double quotes")
	}
	if tokenFrom([]byte(`nothing`)) != "" {
		t.Fatal("no token")
	}
	if sessionFrom(map[string]string{"Set-Cookie": "PHPSESSID=abc; path=/, security=low; path=/"}) != "abc" {
		t.Fatal("session")
	}
	if cookieHeader("", "low") != "security=low" || cookieHeader("s", "") != "PHPSESSID=s" {
		t.Fatal("cookie header")
	}
}
