package finding

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", "/"},
		{"/", "/"},
		{"/users", "/users"},
		{"/Users/", "/users"},
		{"/users/123", "/users/{id}"},
		{"/users/123/", "/users/{id}"},
		{"/users/123/posts/456", "/users/{id}/posts/{id}"},
		{"/a/1/2/3", "/a/{id}/{id}/{id}"},
		{"/users/42?x=1", "/users/{id}"},
		{"/users/42#frag", "/users/{id}"},
		{"/users/42?x=1#frag", "/users/{id}"},
		{"https://Example.com/API/v1/Items/9", "/api/v1/items/{id}"},
		{"http://example.com:8080/items/9/", "/items/{id}"},
		{"https://example.com", "/"},
		{"//example.com/x/1", "/x/{id}"},
		{"/orders/550e8400-e29b-41d4-a716-446655440000", "/orders/{uuid}"},
		{"/orders/550E8400-E29B-41D4-A716-446655440000/items/7", "/orders/{uuid}/items/{id}"},
		{"/double//slashes///x", "/double/slashes/x"},
		{"//host//slashes///x", "/slashes/x"},
		{"users/5", "/users/{id}"},
		{"/v1/users", "/v1/users"},
		{"/user123/x", "/user123/x"},
		{"/123", "/{id}"},
		{"/123/", "/{id}"},
		{"  /trim/1  ", "/trim/{id}"},
		{"/a/b/c/", "/a/b/c"},
		{"/////", "/"},
		{"ftp://h/p/1", "/p/{id}"},
		{"/path?only=query", "/path"},
		{"?x=1", "/"},
		{"/Ünïcode/1", "/ünïcode/{id}"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := NormalizePath(tt.in); got != tt.want {
				t.Fatalf("NormalizePath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizePathIdempotent(t *testing.T) {
	for _, in := range []string{"/users/123/posts/456", "https://x.y/a/1", "/{id}/{uuid}"} {
		once := NormalizePath(in)
		if twice := NormalizePath(once); twice != once {
			t.Fatalf("not idempotent: %q -> %q -> %q", in, once, twice)
		}
	}
}

func TestDedupKeyCanonical(t *testing.T) {
	want := sha256.Sum256([]byte(`{"class":"sqli","lab":"lab-a","method":"GET","param":"id","path_template":"/users/{id}"}`))
	got := DedupKey("lab-a", model.ClassSQLi, "get", "/users/42?x=1", "id")
	if got != hex.EncodeToString(want[:]) {
		t.Fatalf("DedupKey = %s, want %s", got, hex.EncodeToString(want[:]))
	}
	if len(got) != 64 {
		t.Fatalf("len = %d", len(got))
	}
}

func TestDedupKeyStability(t *testing.T) {
	tests := []struct {
		name     string
		a, b     [5]string
		wantSame bool
	}{
		{"identical", [5]string{"l", "idor", "GET", "/u/1", "id"}, [5]string{"l", "idor", "GET", "/u/1", "id"}, true},
		{"method case", [5]string{"l", "idor", "get", "/u/1", "id"}, [5]string{"l", "idor", "GET", "/u/1", "id"}, true},
		{"different ids", [5]string{"l", "idor", "GET", "/u/1", "id"}, [5]string{"l", "idor", "GET", "/u/99", "id"}, true},
		{"query stripped", [5]string{"l", "idor", "GET", "/u/1?a=b", "id"}, [5]string{"l", "idor", "GET", "/u/1", "id"}, true},
		{"different lab", [5]string{"l1", "idor", "GET", "/u/1", "id"}, [5]string{"l2", "idor", "GET", "/u/1", "id"}, false},
		{"different class", [5]string{"l", "idor", "GET", "/u/1", "id"}, [5]string{"l", "sqli", "GET", "/u/1", "id"}, false},
		{"different method", [5]string{"l", "idor", "GET", "/u/1", "id"}, [5]string{"l", "idor", "POST", "/u/1", "id"}, false},
		{"different param", [5]string{"l", "idor", "GET", "/u/1", "id"}, [5]string{"l", "idor", "GET", "/u/1", "uid"}, false},
		{"different path", [5]string{"l", "idor", "GET", "/u/1", "id"}, [5]string{"l", "idor", "GET", "/v/1", "id"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ka := DedupKey(tt.a[0], model.Class(tt.a[1]), tt.a[2], tt.a[3], tt.a[4])
			kb := DedupKey(tt.b[0], model.Class(tt.b[1]), tt.b[2], tt.b[3], tt.b[4])
			if (ka == kb) != tt.wantSame {
				t.Fatalf("same=%v want %v (%s vs %s)", ka == kb, tt.wantSame, ka, kb)
			}
		})
	}
}

func TestValidClass(t *testing.T) {
	for _, c := range model.AllClasses {
		if got, ok := ValidClass(string(c)); !ok || got != c {
			t.Fatalf("ValidClass(%q) = %q,%v", c, got, ok)
		}
	}
	if got, ok := ValidClass(" SQLi "); !ok || got != model.ClassSQLi {
		t.Fatalf("case/space insensitive: %q %v", got, ok)
	}
	for _, bad := range []string{"", "rce", "sql-injection", "xss"} {
		if _, ok := ValidClass(bad); ok {
			t.Fatalf("ValidClass(%q) accepted", bad)
		}
	}
}

func TestValidSeverity(t *testing.T) {
	for _, s := range []string{"info", "low", "medium", "high", "critical", "HIGH"} {
		if _, ok := ValidSeverity(s); !ok {
			t.Fatalf("ValidSeverity(%q) rejected", s)
		}
	}
	if _, ok := ValidSeverity("severe"); ok {
		t.Fatal("ValidSeverity(severe) accepted")
	}
}

func TestNewFinding(t *testing.T) {
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	long := strings.Repeat("é", 300)
	f, err := NewFinding(Params{
		RunID: "run-1", Lab: "lab-a", Class: model.ClassIDOR, Method: "get",
		Path: "https://h/api/users/77?x=1", Param: "id", Severity: model.SevHigh,
		Summary: long, Model: model.ModelRef{Provider: "fake", Model: "m", Agent: "probe"},
		ToolCallIDs: []string{"tc-b", "tc-a"}, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantKey := DedupKey("lab-a", model.ClassIDOR, "GET", "/api/users/{id}", "id")
	if f.DedupKey != wantKey {
		t.Fatalf("DedupKey = %s want %s", f.DedupKey, wantKey)
	}
	if f.ID != "f-"+wantKey[:16] {
		t.Fatalf("ID = %s", f.ID)
	}
	if f.State != model.StateTheorized {
		t.Fatalf("State = %s", f.State)
	}
	if f.Location.Method != "GET" || f.Location.PathTemplate != "/api/users/{id}" || f.Location.Param != "id" {
		t.Fatalf("Location = %+v", f.Location)
	}
	if n := len([]rune(f.Summary)); n != MaxSummaryRunes {
		t.Fatalf("summary runes = %d", n)
	}
	if f.ToolCallIDs[0] != "tc-a" || f.ToolCallIDs[1] != "tc-b" {
		t.Fatalf("ToolCallIDs not sorted: %v", f.ToolCallIDs)
	}
	if !f.FirstSeen.Equal(now) || !f.LastSeen.Equal(now) {
		t.Fatalf("times = %v %v", f.FirstSeen, f.LastSeen)
	}
	if _, err := NewFinding(Params{Class: "rce"}); !errors.Is(err, ErrInvalidClass) {
		t.Fatalf("invalid class err = %v", err)
	}
	g, _ := NewFinding(Params{Class: model.ClassSQLi, Severity: "bogus"})
	if g.Severity != model.SevInfo {
		t.Fatalf("bogus severity -> %s", g.Severity)
	}
	if g.FirstSeen.IsZero() {
		t.Fatal("zero Now not defaulted")
	}
}

func TestMerge(t *testing.T) {
	t1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := t1.Add(time.Hour)
	t3 := t1.Add(2 * time.Hour)
	ev1 := model.Evidence{Kind: "http-exchange", StatusCode: 200, ToolCallID: "a"}
	ev2 := model.Evidence{Kind: "http-exchange", StatusCode: 500, ToolCallID: "b"}
	existing := model.Finding{ID: "f-1", Severity: model.SevLow, State: model.StateTheorized,
		FirstSeen: t2, LastSeen: t2, Evidence: []model.Evidence{ev1}, ToolCallIDs: []string{"a"},
		ControlPlane: []model.AuditRef{{RunID: "r", Seq: 2}}}
	incoming := model.Finding{ID: "f-other", Severity: model.SevHigh, State: model.StateValidated,
		FirstSeen: t1, LastSeen: t3, Evidence: []model.Evidence{ev1, ev2}, ToolCallIDs: []string{"b", "a"},
		ControlPlane: []model.AuditRef{{RunID: "r", Seq: 1}, {RunID: "r", Seq: 2}}}
	m := Merge(existing, incoming)
	if m.ID != "f-1" {
		t.Fatalf("identity changed: %s", m.ID)
	}
	if !m.FirstSeen.Equal(t1) || !m.LastSeen.Equal(t3) {
		t.Fatalf("times = %v %v", m.FirstSeen, m.LastSeen)
	}
	if len(m.Evidence) != 2 {
		t.Fatalf("evidence = %v", m.Evidence)
	}
	if m.Severity != model.SevHigh {
		t.Fatalf("severity = %s", m.Severity)
	}
	if m.State != model.StateValidated {
		t.Fatalf("state = %s", m.State)
	}
	if len(m.ToolCallIDs) != 2 || m.ToolCallIDs[0] != "a" {
		t.Fatalf("tool ids = %v", m.ToolCallIDs)
	}
	if len(m.ControlPlane) != 2 || m.ControlPlane[0].Seq != 1 {
		t.Fatalf("control plane = %v", m.ControlPlane)
	}
	// Merging a theorized duplicate into a refuted finding keeps refuted.
	ref := model.Finding{State: model.StateRefuted, FirstSeen: t1, LastSeen: t1}
	if got := Merge(ref, model.Finding{State: model.StateTheorized, FirstSeen: t2}); got.State != model.StateRefuted {
		t.Fatalf("state = %s", got.State)
	}
}

func TestTransition(t *testing.T) {
	tests := []struct {
		from, to model.FindingState
		ok       bool
	}{
		{model.StateTheorized, model.StateValidated, true},
		{model.StateTheorized, model.StateRefuted, true},
		{model.StateTheorized, model.StateDeclined, true},
		{model.StateTheorized, model.StateTheorized, false},
		{model.StateTheorized, "bogus", false},
		{model.StateValidated, model.StateRefuted, false},
		{model.StateRefuted, model.StateValidated, false},
		{model.StateDeclined, model.StateTheorized, false},
		{model.StateValidated, model.StateTheorized, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			f := model.Finding{State: tt.from}
			err := Transition(&f, tt.to)
			if tt.ok {
				if err != nil || f.State != tt.to {
					t.Fatalf("err=%v state=%s", err, f.State)
				}
				return
			}
			if !errors.Is(err, ErrInvalidTransition) {
				t.Fatalf("err = %v", err)
			}
			if f.State != tt.from {
				t.Fatalf("state changed on failure: %s", f.State)
			}
		})
	}
	if err := Transition(nil, model.StateValidated); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("nil: %v", err)
	}
}

func TestTruncateSummary(t *testing.T) {
	if got := TruncateSummary("  short  "); got != "short" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("ab", 200)
	if got := TruncateSummary(long); len([]rune(got)) != MaxSummaryRunes {
		t.Fatalf("len = %d", len([]rune(got)))
	}
}
