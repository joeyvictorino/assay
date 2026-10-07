package syntheticops

import (
	"context"
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

// fakeLab mimics the real lab's /healthz and /api/login shapes.
func fakeLab(t *testing.T) (*httptest.Server, *httpx.Client) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		json.NewDecoder(r.Body).Decode(&in)
		if in["username"] == "alice" && in["password"] == "Wonderland-1" {
			w.Write([]byte(`{"token":"tok-alice","user_id":2,"role":"user"}`))
			return
		}
		w.WriteHeader(401)
		w.Write([]byte(`{"error":"invalid credentials"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return srv, &httpx.Client{Gate: hostGate{u.Host}, Auditor: &httpx.MemAuditor{}, RunID: "t"}
}

func TestHealthAndLogin(t *testing.T) {
	srv, client := fakeLab(t)
	lab := New(srv.URL+"/", "../../../labs/synthetic-ops", client)
	if lab.Name() != "synthetic-ops" || lab.BaseURL() != srv.URL {
		t.Fatalf("name/base: %s %s", lab.Name(), lab.BaseURL())
	}
	ctx := context.Background()
	if err := lab.Health(ctx); err != nil {
		t.Fatal(err)
	}
	if err := lab.Setup(ctx); err != nil {
		t.Fatal(err)
	}
	hdrs, id, err := lab.Login(ctx, labs.Account{Username: "alice", Password: "Wonderland-1"})
	if err != nil || hdrs["Authorization"] != "Bearer tok-alice" || id != "2" {
		t.Fatalf("login: %v %v %s", hdrs, err, id)
	}
	if _, _, err := lab.Login(ctx, labs.Account{Username: "alice", Password: "nope"}); err == nil {
		t.Fatal("bad password must fail")
	}
	accts := lab.Accounts()
	if len(accts) != 3 || accts[0].Role != "admin" || accts[0].Password != "admin123" {
		t.Fatalf("accounts %+v", accts)
	}
	accts[0].Password = "mutated"
	if lab.Accounts()[0].Password != "admin123" {
		t.Fatal("Accounts must return a copy")
	}
}

func TestGroundTruthFromRepo(t *testing.T) {
	_, client := fakeLab(t)
	lab := New("http://127.0.0.1:9000", "../../../labs/synthetic-ops", client)
	entries, exhaustive, err := labs.LoadGroundTruth("../../../labs/synthetic-ops/ground_truth.json")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := lab.GroundTruth()
	if !ok || !exhaustive || len(got) != len(entries) {
		t.Fatalf("ok=%v exhaustive=%v n=%d", ok, exhaustive, len(got))
	}
	if e := labs.FindByClass(got, model.ClassAuthMissing, "/api/admin/stats"); e == nil || !e.AuthRequired {
		t.Fatalf("auth-missing entry must be marked auth_required: %+v", e)
	}
	if e := labs.FindByClass(got, model.ClassInfoDisclosure, "/debug/config"); e == nil || e.Marker != SecretMarker {
		t.Fatalf("info-disclosure marker: %+v", e)
	}
	for _, e := range got {
		if e.SourceRef == "" {
			t.Errorf("%s: missing source_ref", e.ID)
		}
	}
}

func TestGroundTruthMissingFileIsNotExhaustive(t *testing.T) {
	_, client := fakeLab(t)
	lab := New("http://127.0.0.1:9000", t.TempDir(), client)
	if got, ok := lab.GroundTruth(); ok || got != nil {
		t.Fatal("missing ground truth must report false")
	}
}

func TestHealthFailsOnDenial(t *testing.T) {
	srv, _ := fakeLab(t)
	denyAll := &httpx.Client{Gate: hostGate{"nobody.invalid"}, Auditor: &httpx.MemAuditor{}}
	lab := New(srv.URL, t.TempDir(), denyAll)
	if err := lab.Health(context.Background()); err == nil {
		t.Fatal("gate denial must surface")
	}
}
