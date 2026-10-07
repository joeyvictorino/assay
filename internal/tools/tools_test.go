package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/joeyvictorino/assay/internal/finding"
	"github.com/joeyvictorino/assay/internal/model"
)

type gate struct{ allow bool }

func (g gate) Allow(_ context.Context, _ string) model.Decision {
	if g.allow {
		return model.Decision{Effect: model.EffectAllow, Reason: "ALLOW_SCOPE"}
	}
	return model.Decision{Effect: model.EffectDeny, Reason: "DENY_OUT_OF_SCOPE"}
}

type httpCall struct {
	method, url string
	headers     map[string]string
	body        []byte
}

func fakeHTTP(status int, headers map[string]string, body string, calls *[]httpCall) HTTPFunc {
	return func(_ context.Context, method, url string, h map[string]string, b []byte) (int, map[string]string, []byte, error) {
		if calls != nil {
			*calls = append(*calls, httpCall{method, url, h, b})
		}
		return status, headers, []byte(body), nil
	}
}

func env(h HTTPFunc, allow bool) Env {
	return Env{
		HTTP: h, Gate: gate{allow}, Lab: "lab-a", RunID: "run-1",
		Model: model.ModelRef{Provider: "fake", Model: "m", Agent: "probe"},
		Now:   func() time.Time { return time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC) },
	}
}

func decode(t *testing.T, r model.ToolResult) map[string]any {
	t.Helper()
	if r.IsError {
		t.Fatalf("unexpected error result: %s", r.Content)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(r.Content), &m); err != nil {
		t.Fatalf("result not JSON: %v: %s", err, r.Content)
	}
	return m
}

func TestManifestsLoadAndValidate(t *testing.T) {
	ms, err := Manifests()
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != len(Names) {
		t.Fatalf("got %d manifests, want %d", len(ms), len(Names))
	}
	want := map[string]struct {
		tier model.RiskTier
		caps []string
		side bool
	}{
		"http_get":        {model.TierMedium, []string{"http.read"}, false},
		"http_post":       {model.TierHigh, []string{"http.write"}, true},
		"inspect_headers": {model.TierLow, []string{"http.read"}, false},
		"login_as":        {model.TierHigh, []string{"auth.login", "http.write"}, true},
		"report_finding":  {model.TierLow, []string{"report"}, false},
		"score_task":      {model.TierLow, []string{"score"}, false},
		"delegate":        {model.TierMedium, []string{"delegate"}, false},
	}
	for i, m := range ms {
		if m.Name != Names[i] {
			t.Fatalf("order: %s at %d, want %s", m.Name, i, Names[i])
		}
		w, ok := want[m.Name]
		if !ok {
			t.Fatalf("unexpected manifest %s", m.Name)
		}
		if m.RiskTier != w.tier || m.SideEffects != w.side || strings.Join(m.Capabilities, ",") != strings.Join(w.caps, ",") {
			t.Fatalf("%s: tier=%s caps=%v side=%v", m.Name, m.RiskTier, m.Capabilities, m.SideEffects)
		}
		if m.Version != "1" || m.Signature != "" || m.Signer != "" {
			t.Fatalf("%s: version=%q signed=%v", m.Name, m.Version, m.Signature != "")
		}
		if len(m.AllowedAgents) == 0 {
			t.Fatalf("%s: no allowed agents", m.Name)
		}
		if err := ValidateManifest(m); err != nil {
			t.Fatal(err)
		}
	}
	// report_finding enums mirror the taxonomy exactly.
	rf, err := Manifest("report_finding")
	if err != nil {
		t.Fatal(err)
	}
	props := rf.InputSchema["properties"].(map[string]any)
	classEnum := props["class"].(map[string]any)["enum"].([]any)
	if len(classEnum) != len(model.AllClasses) {
		t.Fatalf("class enum has %d entries, taxonomy has %d", len(classEnum), len(model.AllClasses))
	}
	for i, c := range model.AllClasses {
		if classEnum[i] != string(c) {
			t.Fatalf("class enum[%d] = %v, want %s", i, classEnum[i], c)
		}
	}
	if props["summary"].(map[string]any)["maxLength"] != float64(280) {
		t.Fatal("summary maxLength must be 280")
	}
	if _, err := Manifest("nope"); err == nil {
		t.Fatal("unknown manifest accepted")
	}
}

func TestValidateManifestRejects(t *testing.T) {
	good := func() model.ToolManifest {
		return model.ToolManifest{Name: "t", Version: "1", Description: "d", RiskTier: model.TierLow,
			Capabilities: []string{"x"}, TimeoutMS: 1,
			InputSchema: map[string]any{"type": "object", "additionalProperties": false,
				"properties": map[string]any{"a": map[string]any{"type": "string"}}, "required": []any{"a"}}}
	}
	tests := []struct {
		name string
		mut  func(*model.ToolManifest)
	}{
		{"no name", func(m *model.ToolManifest) { m.Name = "" }},
		{"no version", func(m *model.ToolManifest) { m.Version = "" }},
		{"no description", func(m *model.ToolManifest) { m.Description = "" }},
		{"bad tier", func(m *model.ToolManifest) { m.RiskTier = "extreme" }},
		{"no caps", func(m *model.ToolManifest) { m.Capabilities = nil }},
		{"zero timeout", func(m *model.ToolManifest) { m.TimeoutMS = 0 }},
		{"nil schema", func(m *model.ToolManifest) { m.InputSchema = nil }},
		{"not object", func(m *model.ToolManifest) { m.InputSchema["type"] = "string" }},
		{"additionalProperties true", func(m *model.ToolManifest) { m.InputSchema["additionalProperties"] = true }},
		{"additionalProperties missing", func(m *model.ToolManifest) { delete(m.InputSchema, "additionalProperties") }},
		{"required missing", func(m *model.ToolManifest) { delete(m.InputSchema, "required") }},
		{"required not property", func(m *model.ToolManifest) { m.InputSchema["required"] = []any{"zzz"} }},
		{"optional property", func(m *model.ToolManifest) { m.InputSchema["required"] = []any{} }},
	}
	if err := ValidateManifest(good()); err != nil {
		t.Fatalf("baseline invalid: %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := good()
			tt.mut(&m)
			if err := ValidateManifest(m); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestHTTPGet(t *testing.T) {
	var calls []httpCall
	e := env(fakeHTTP(200, map[string]string{"Server": "nginx", "Set-Cookie": "sid=secret123; HttpOnly; Path=/", "Authorization": "Bearer x"}, "hello body", &calls), true)
	e.Sanitize = func(s string) string { return strings.ReplaceAll(s, "hello", "[redacted]") }
	r, err := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c1", Name: "http_get", Args: map[string]any{"url": "http://lab.local/x/1"}}, e)
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, r)
	if r.ToolCallID != "c1" || m["status"] != float64(200) || m["body"] != "[redacted] body" || m["body_bytes"] != float64(10) {
		t.Fatalf("result = %v", m)
	}
	h := m["headers"].(map[string]any)
	if h["server"] != "nginx" || h["set-cookie"] != "sid=[redacted]; HttpOnly; Path=/" || h["authorization"] != "[redacted]" {
		t.Fatalf("headers = %v", h)
	}
	if len(calls) != 1 || calls[0].method != "GET" || calls[0].url != "http://lab.local/x/1" || calls[0].body != nil {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestHTTPBodyCap(t *testing.T) {
	big := strings.Repeat("A", MaxBodyBytes+100)
	e := env(fakeHTTP(200, nil, big, nil), true)
	r, _ := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c", Name: "http_get", Args: map[string]any{"url": "http://lab.local/"}}, e)
	m := decode(t, r)
	if len(m["body"].(string)) != MaxBodyBytes || m["body_truncated"] != true || m["body_bytes"] != float64(MaxBodyBytes+100) {
		t.Fatalf("cap not applied: len=%d truncated=%v", len(m["body"].(string)), m["body_truncated"])
	}
}

func TestGateDenies(t *testing.T) {
	var calls []httpCall
	for _, name := range []string{"http_get", "inspect_headers"} {
		e := env(fakeHTTP(200, nil, "", &calls), false)
		r, err := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c", Name: name, Args: map[string]any{"url": "http://evil/"}}, e)
		if err != nil {
			t.Fatal(err)
		}
		if !r.IsError || !strings.Contains(r.Content, "gate denied: DENY_OUT_OF_SCOPE") {
			t.Fatalf("%s: result = %+v", name, r)
		}
	}
	e := env(fakeHTTP(200, nil, "", &calls), false)
	r, _ := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c", Name: "http_post", Args: map[string]any{"url": "http://evil/", "content_type": "text/plain", "body": "x"}}, e)
	if !r.IsError {
		t.Fatal("http_post not denied")
	}
	r, _ = NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c", Name: "login_as", Args: map[string]any{"url": "http://evil/", "username": "u", "password": "p"}}, e)
	if !r.IsError {
		t.Fatal("login_as not denied")
	}
	if len(calls) != 0 {
		t.Fatalf("HTTP reached despite gate denial: %+v", calls)
	}
	// nil gate fails closed.
	e = env(fakeHTTP(200, nil, "", &calls), true)
	e.Gate = nil
	r, _ = NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c", Name: "http_get", Args: map[string]any{"url": "http://lab.local/"}}, e)
	if !r.IsError || len(calls) != 0 {
		t.Fatalf("nil gate did not fail closed: %+v", r)
	}
}

func TestURLValidation(t *testing.T) {
	for _, u := range []string{"", "lab.local/x", "ftp://lab.local/x", "/relative", "javascript:alert(1)"} {
		e := env(fakeHTTP(200, nil, "", nil), true)
		r, _ := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c", Name: "http_get", Args: map[string]any{"url": u}}, e)
		if !r.IsError {
			t.Fatalf("url %q accepted", u)
		}
	}
	e := env(fakeHTTP(200, nil, "", nil), true)
	r, _ := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c", Name: "http_get", Args: map[string]any{}}, e)
	if !r.IsError || !strings.Contains(r.Content, "missing argument: url") {
		t.Fatalf("missing url: %+v", r)
	}
	r, _ = NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c", Name: "http_get", Args: map[string]any{"url": 5}}, e)
	if !r.IsError {
		t.Fatalf("non-string url accepted: %+v", r)
	}
}

func TestHTTPPost(t *testing.T) {
	var calls []httpCall
	e := env(fakeHTTP(201, map[string]string{"Location": "/items/9"}, `{"id":9}`, &calls), true)
	r, _ := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c", Name: "http_post", Args: map[string]any{"url": "http://lab.local/items", "content_type": "application/json", "body": `{"a":1}`}}, e)
	m := decode(t, r)
	if m["status"] != float64(201) || m["body"] != `{"id":9}` {
		t.Fatalf("result = %v", m)
	}
	if calls[0].method != "POST" || string(calls[0].body) != `{"a":1}` || calls[0].headers["Content-Type"] != "application/json" {
		t.Fatalf("call = %+v", calls[0])
	}
}

func TestInspectHeadersNoBody(t *testing.T) {
	e := env(fakeHTTP(200, map[string]string{"X-Frame-Options": "DENY"}, "big body", nil), true)
	r, _ := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c", Name: "inspect_headers", Args: map[string]any{"url": "http://lab.local/"}}, e)
	m := decode(t, r)
	if _, has := m["body"]; has {
		t.Fatal("inspect_headers must not return a body")
	}
	if m["headers"].(map[string]any)["x-frame-options"] != "DENY" || m["body_bytes"] != float64(8) {
		t.Fatalf("result = %v", m)
	}
}

func TestLoginAs(t *testing.T) {
	var calls []httpCall
	e := env(fakeHTTP(302, map[string]string{"Set-Cookie": "session=abc; HttpOnly", "Location": "/home"}, "welcome, password was hunter2", &calls), true)
	r, _ := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c", Name: "login_as", Args: map[string]any{"url": "http://lab.local/login", "username": "alice", "password": "hunter2"}}, e)
	m := decode(t, r)
	if m["status"] != float64(302) || m["session_cookie_set"] != true || m["location"] != "/home" {
		t.Fatalf("result = %v", m)
	}
	if strings.Contains(r.Content, "hunter2") || strings.Contains(r.Content, "abc") {
		t.Fatalf("secret echoed: %s", r.Content)
	}
	if string(calls[0].body) != "password=hunter2&username=alice" || calls[0].headers["Content-Type"] != "application/x-www-form-urlencoded" {
		t.Fatalf("call = %+v", calls[0])
	}
}

func TestHTTPErrorFromClient(t *testing.T) {
	e := env(func(context.Context, string, string, map[string]string, []byte) (int, map[string]string, []byte, error) {
		return 0, nil, nil, errors.New("dial tcp: refused")
	}, true)
	r, err := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c", Name: "http_get", Args: map[string]any{"url": "http://lab.local/"}}, e)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsError || !strings.Contains(r.Content, "request failed") {
		t.Fatalf("result = %+v", r)
	}
	e.HTTP = nil
	r, _ = NewExecutor().Execute(context.Background(), model.ToolCall{ID: "c", Name: "http_get", Args: map[string]any{"url": "http://lab.local/"}}, e)
	if !r.IsError {
		t.Fatal("nil HTTP must be an error result")
	}
}

func TestReportFinding(t *testing.T) {
	var got []model.Finding
	e := env(nil, true)
	e.Report = func(f model.Finding) { got = append(got, f) }
	e.Sanitize = func(s string) string { return strings.ReplaceAll(s, "SECRET", "[redacted]") }
	args := map[string]any{
		"class": "idor", "method": "get", "path": "https://lab.local/api/users/42?x=1", "param": "id",
		"severity": "high", "summary": "User 42 readable as another user SECRET " + strings.Repeat("z", 300),
		"evidence_tool_call_ids": []any{"tc-2", "tc-1"},
	}
	r, err := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "rf", Name: "report_finding", Args: args}, e)
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, r)
	if len(got) != 1 {
		t.Fatalf("Report called %d times", len(got))
	}
	f := got[0]
	wantKey := finding.DedupKey("lab-a", model.ClassIDOR, "GET", "/api/users/{id}", "id")
	if f.DedupKey != wantKey || m["dedup_key"] != wantKey || m["id"] != f.ID || m["state"] != "theorized" {
		t.Fatalf("finding = %+v result = %v", f, m)
	}
	if f.Location.PathTemplate != "/api/users/{id}" || f.Location.Method != "GET" || f.Severity != model.SevHigh {
		t.Fatalf("location = %+v sev = %s", f.Location, f.Severity)
	}
	if strings.Contains(f.Summary, "SECRET") || len([]rune(f.Summary)) != finding.MaxSummaryRunes {
		t.Fatalf("summary = %q", f.Summary)
	}
	if f.ToolCallIDs[0] != "tc-1" || f.Lab != "lab-a" || f.RunID != "run-1" || f.Model.Agent != "probe" {
		t.Fatalf("finding = %+v", f)
	}
	if !f.FirstSeen.Equal(e.Now()) {
		t.Fatalf("FirstSeen = %v", f.FirstSeen)
	}

	bad := []struct {
		name string
		mut  func(map[string]any)
		want string
	}{
		{"class", func(a map[string]any) { a["class"] = "rce" }, "class must be"},
		{"method", func(a map[string]any) { a["method"] = "HEAD" }, "method must be"},
		{"severity", func(a map[string]any) { a["severity"] = "urgent" }, "severity must be"},
		{"ids", func(a map[string]any) { a["evidence_tool_call_ids"] = []any{1} }, "array of strings"},
		{"summary", func(a map[string]any) { delete(a, "summary") }, "missing argument: summary"},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			a := map[string]any{}
			for k, v := range args {
				a[k] = v
			}
			tt.mut(a)
			n := len(got)
			r, _ := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "rf", Name: "report_finding", Args: a}, e)
			if !r.IsError || !strings.Contains(r.Content, tt.want) {
				t.Fatalf("result = %+v", r)
			}
			if len(got) != n {
				t.Fatal("invalid finding reported")
			}
		})
	}
}

func TestScoreTask(t *testing.T) {
	var gotScore int
	var gotCritique string
	e := env(nil, true)
	e.Score = func(s int, c string) { gotScore, gotCritique = s, c }
	r, _ := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "s", Name: "score_task", Args: map[string]any{"score": float64(7), "critique": "fine"}}, e)
	m := decode(t, r)
	if m["recorded"] != true || m["score"] != float64(7) || gotScore != 7 || gotCritique != "fine" {
		t.Fatalf("result = %v score=%d critique=%q", m, gotScore, gotCritique)
	}
	for _, v := range []any{float64(11), float64(-1), float64(7.5), "7", nil} {
		args := map[string]any{"critique": "x"}
		if v != nil {
			args["score"] = v
		}
		r, _ := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "s", Name: "score_task", Args: args}, e)
		if !r.IsError {
			t.Fatalf("score %v accepted", v)
		}
	}
}

func TestDelegate(t *testing.T) {
	e := env(nil, true)
	r, _ := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "d", Name: "delegate", Args: map[string]any{"agent": "probe", "objective": "look"}}, e)
	if !r.IsError || !strings.Contains(r.Content, "not available") {
		t.Fatalf("result = %+v", r)
	}
	e.Delegate = func(_ context.Context, agent, objective string) (string, error) {
		return "delegate " + agent + " did " + objective + strings.Repeat("!", MaxBodyBytes), nil
	}
	r, _ = NewExecutor().Execute(context.Background(), model.ToolCall{ID: "d", Name: "delegate", Args: map[string]any{"agent": "probe", "objective": "look"}}, e)
	m := decode(t, r)
	if m["agent"] != "probe" || len(m["result"].(string)) != MaxBodyBytes {
		t.Fatalf("result = %v", m)
	}
	e.Delegate = func(context.Context, string, string) (string, error) { return "", errors.New("depth exceeded") }
	r, _ = NewExecutor().Execute(context.Background(), model.ToolCall{ID: "d", Name: "delegate", Args: map[string]any{"agent": "probe", "objective": "look"}}, e)
	if !r.IsError || !strings.Contains(r.Content, "depth exceeded") {
		t.Fatalf("result = %+v", r)
	}
}

func TestUnknownToolAndNilArgs(t *testing.T) {
	e := env(nil, true)
	r, err := NewExecutor().Execute(context.Background(), model.ToolCall{ID: "u", Name: "rm_rf"}, e)
	if err != nil {
		t.Fatal(err)
	}
	if !r.IsError || !strings.Contains(r.Content, "unknown tool") || r.ToolCallID != "u" {
		t.Fatalf("result = %+v", r)
	}
	r, _ = NewExecutor().Execute(context.Background(), model.ToolCall{ID: "u", Name: "score_task"}, e)
	if !r.IsError {
		t.Fatal("nil args must produce an error result, not a panic")
	}
}

func TestRedactCookie(t *testing.T) {
	tests := []struct{ in, want string }{
		{"a=b", "a=[redacted]"},
		{"a=b; HttpOnly; Secure", "a=[redacted]; HttpOnly; Secure"},
		{"a=b;Path=/ ; SameSite=Lax", "a=[redacted]; Path=/; SameSite=Lax"},
		{"novalue", "novalue=[redacted]"},
	}
	for _, tt := range tests {
		if got := redactCookie(tt.in); got != tt.want {
			t.Fatalf("redactCookie(%q) = %q want %q", tt.in, got, tt.want)
		}
	}
}
