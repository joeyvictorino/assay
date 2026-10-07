package juiceshop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func fakeJWT(userID int) string {
	payload, _ := json.Marshal(map[string]any{"data": map[string]any{"id": userID, "email": "x"}})
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"RS256"}`)) + "." + enc(payload) + ".sig"
}

func fakeShop(t *testing.T) (*httptest.Server, *httpx.Client) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /rest/admin/application-version", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"version":"17.0.0"}`))
	})
	mux.HandleFunc("POST /rest/user/login", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		json.NewDecoder(r.Body).Decode(&in)
		if in["email"] == "admin@juice-sh.op" && in["password"] == "admin123" {
			json.NewEncoder(w).Encode(map[string]any{"authentication": map[string]any{"token": fakeJWT(1), "bid": 7, "umail": in["email"]}})
			return
		}
		w.WriteHeader(401)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return srv, &httpx.Client{Gate: hostGate{u.Host}, Auditor: &httpx.MemAuditor{}}
}

func TestAdapter(t *testing.T) {
	srv, client := fakeShop(t)
	lab := New(srv.URL, "../../../labs/juice-shop/accounts.yaml", "../../../labs/juice-shop/reference_findings.json", client)
	ctx := context.Background()
	if lab.Name() != Name || lab.BaseURL() != srv.URL {
		t.Fatal("identity")
	}
	if err := lab.Health(ctx); err != nil {
		t.Fatal(err)
	}
	if err := lab.Setup(ctx); err != nil {
		t.Fatal(err)
	}
	accts := lab.Accounts()
	if len(accts) < 3 || accts[0].Role != "admin" || accts[0].Username != "admin@juice-sh.op" || accts[0].Password != "admin123" {
		t.Fatalf("accounts %+v", accts)
	}
	refs, exhaustive := lab.GroundTruth()
	if exhaustive || len(refs) < 8 || len(refs) > 12 {
		t.Fatalf("reference list: exhaustive=%v n=%d", exhaustive, len(refs))
	}
	for _, r := range refs {
		if r.Lab != Name {
			t.Errorf("%s: lab %q", r.ID, r.Lab)
		}
	}
	hdrs, id, err := lab.Login(ctx, accts[0])
	if err != nil || id != "1" || hdrs["Authorization"] == "" || hdrs["Cookie"] == "" {
		t.Fatalf("login: %v %s %v", hdrs, id, err)
	}
	if _, _, err := lab.Login(ctx, labs.Account{Username: "nobody@juice-sh.op", Password: "x"}); err == nil {
		t.Fatal("bad login must fail")
	}
}

func TestMissingFiles(t *testing.T) {
	_, client := fakeShop(t)
	lab := New("http://127.0.0.1:3000", "/nonexistent/accounts.yaml", "/nonexistent/ref.json", client)
	if len(lab.Accounts()) != 0 {
		t.Fatal("no accounts expected")
	}
	if _, ok := lab.GroundTruth(); ok {
		t.Fatal("must not claim ground truth")
	}
}

func TestJWTUserID(t *testing.T) {
	if got := jwtUserID(fakeJWT(42)); got != "42" {
		t.Fatalf("got %q", got)
	}
	if got := jwtUserID("not.a.jwt.really"); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := jwtUserID("a.b"); got != "" {
		t.Fatalf("got %q", got)
	}
}
