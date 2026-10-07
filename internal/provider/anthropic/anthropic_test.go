package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/provider"
)

type capture struct {
	mu   sync.Mutex
	body map[string]any
	hdr  http.Header
}

func newServer(t *testing.T, status int, body string, cap *capture) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cap != nil {
			b, _ := io.ReadAll(r.Body)
			cap.mu.Lock()
			cap.hdr = r.Header.Clone()
			_ = json.Unmarshal(b, &cap.body)
			cap.mu.Unlock()
		}
		w.Header().Set("request-id", "req_test_123")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

func newProvider(url string) *Provider {
	return New(Config{APIKey: "test-key", BaseURL: url})
}

var tools = []model.ToolManifest{{
	Name:        "report_finding",
	Description: "report",
	InputSchema: map[string]any{
		"type":                 "object",
		"properties":           map[string]any{"class": map[string]any{"type": "string"}},
		"required":             []any{"class"},
		"additionalProperties": false,
	},
}}

func TestCompleteText(t *testing.T) {
	cap := &capture{}
	srv := newServer(t, 200, `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5-5",
	 "content":[{"type":"text","text":"hello "},{"type":"text","text":"world"}],
	 "stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":7,"cache_creation_input_tokens":3}}`, cap)
	defer srv.Close()
	p := newProvider(srv.URL)
	resp, err := p.Complete(context.Background(), model.Request{
		Model: "claude-sonnet-5-5", System: "sys", MaxTokens: 100, Tools: tools,
		Messages: []model.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "hello world" || resp.StopReason != "end_turn" || resp.Declined {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.Usage.InputTokens != 10 || resp.Usage.OutputTokens != 5 || resp.Usage.CacheReadTokens != 7 || resp.Usage.CacheWriteTokens != 3 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
	if resp.RequestID != "req_test_123" {
		t.Fatalf("request id = %q", resp.RequestID)
	}
	if p.Name() != "anthropic" {
		t.Fatalf("name = %s", p.Name())
	}

	// Wire shape checks.
	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.hdr.Get("x-api-key") != "test-key" {
		t.Fatalf("api key header = %q", cap.hdr.Get("x-api-key"))
	}
	if cap.body["model"] != "claude-sonnet-5-5" || cap.body["max_tokens"] != float64(100) {
		t.Fatalf("body model/max_tokens = %v %v", cap.body["model"], cap.body["max_tokens"])
	}
	if _, has := cap.body["thinking"]; has {
		t.Fatal("thinking must not be sent")
	}
	if _, has := cap.body["tool_choice"]; has {
		t.Fatal("tool_choice must not be sent")
	}
	sys := cap.body["system"].([]any)
	last := sys[len(sys)-1].(map[string]any)
	if last["text"] != "sys" || last["cache_control"].(map[string]any)["type"] != "ephemeral" {
		t.Fatalf("system block = %v", last)
	}
	tl := cap.body["tools"].([]any)[0].(map[string]any)
	if tl["name"] != "report_finding" || tl["strict"] != true || tl["description"] != "report" {
		t.Fatalf("tool = %v", tl)
	}
	schema := tl["input_schema"].(map[string]any)
	if schema["additionalProperties"] != false || schema["type"] != "object" {
		t.Fatalf("schema = %v", schema)
	}
	if req := schema["required"].([]any); len(req) != 1 || req[0] != "class" {
		t.Fatalf("required = %v", schema["required"])
	}
}

func TestCompleteToolUse(t *testing.T) {
	srv := newServer(t, 200, `{"id":"msg_2","type":"message","role":"assistant","model":"claude-sonnet-5-5",
	 "content":[{"type":"text","text":"calling"},{"type":"tool_use","id":"toolu_1","name":"http_get","input":{"url":"http://lab/x","n":2}}],
	 "stop_reason":"tool_use","usage":{"input_tokens":1,"output_tokens":2}}`, nil)
	defer srv.Close()
	resp, err := newProvider(srv.URL).Complete(context.Background(), model.Request{
		Messages: []model.Message{{Role: "user", Content: "go"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StopReason != "tool_use" || len(resp.ToolCalls) != 1 {
		t.Fatalf("resp = %+v", resp)
	}
	tc := resp.ToolCalls[0]
	if tc.ID != "toolu_1" || tc.Name != "http_get" || tc.Args["url"] != "http://lab/x" || tc.Args["n"] != float64(2) {
		t.Fatalf("tool call = %+v", tc)
	}
	if resp.Text != "calling" {
		t.Fatalf("text = %q", resp.Text)
	}
}

func TestCompleteRefusal(t *testing.T) {
	srv := newServer(t, 200, `{"id":"msg_3","type":"message","role":"assistant","model":"claude-sonnet-5-5",
	 "content":[{"type":"text","text":"should not be read"}],
	 "stop_reason":"refusal","stop_details":{"type":"refusal","category":"cyber"},"usage":{"input_tokens":4,"output_tokens":0}}`, nil)
	defer srv.Close()
	resp, err := newProvider(srv.URL).Complete(context.Background(), model.Request{
		Messages: []model.Message{{Role: "user", Content: "x"}},
	})
	if err != nil {
		t.Fatalf("refusal must not be an error: %v", err)
	}
	if !resp.Declined || resp.StopReason != "refusal" {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.Text != "" || len(resp.ToolCalls) != 0 {
		t.Fatalf("refusal content leaked: %+v", resp)
	}
	if resp.Usage.InputTokens != 4 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

func TestCompleteErrors(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		retryable bool
	}{
		{"429", 429, true},
		{"500", 500, true},
		{"529", 529, true},
		{"400", 400, false},
		{"401", 401, false},
		{"404", 404, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("request-id", "req_err")
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, `{"type":"error","error":{"type":"x","message":"nope"}}`)
			}))
			defer srv.Close()
			_, err := newProvider(srv.URL).Complete(context.Background(), model.Request{
				Messages: []model.Message{{Role: "user", Content: "x"}},
			})
			if err == nil {
				t.Fatal("expected error")
			}
			var se *provider.StatusError
			if !errors.As(err, &se) || se.Code != tt.status {
				t.Fatalf("err = %v (%T)", err, err)
			}
			if se.RequestID != "req_err" {
				t.Fatalf("request id = %q", se.RequestID)
			}
			if provider.Classify(err) != tt.retryable {
				t.Fatalf("Classify = %v want %v", provider.Classify(err), tt.retryable)
			}
			if calls != 1 {
				t.Fatalf("provider retried internally: %d calls", calls)
			}
		})
	}
}

func TestBuildParamsMessages(t *testing.T) {
	req := model.Request{
		System: "base",
		Messages: []model.Message{
			{Role: "system", Content: "extra"},
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "let me look", ToolCalls: []model.ToolCall{{ID: "t1", Name: "http_get", Args: map[string]any{"url": "u"}}, {ID: "t2", Name: "inspect_headers", Args: nil}}},
			{Role: "tool", ToolResults: []model.ToolResult{{ToolCallID: "t1", Content: "ok"}, {ToolCallID: "t2", Content: "denied", IsError: true}}},
		},
	}
	params, err := BuildParams(req)
	if err != nil {
		t.Fatal(err)
	}
	if params.Model != DefaultModel || params.MaxTokens != DefaultMaxTokens {
		t.Fatalf("defaults: %s %d", params.Model, params.MaxTokens)
	}
	if len(params.System) != 1 || params.System[0].Text != "base\n\nextra" {
		t.Fatalf("system = %+v", params.System)
	}
	b, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	_ = json.Unmarshal(b, &wire)
	msgs := wire["messages"].([]any)
	if len(msgs) != 3 {
		t.Fatalf("messages = %d: %s", len(msgs), b)
	}
	asst := msgs[1].(map[string]any)
	if asst["role"] != "assistant" {
		t.Fatalf("role = %v", asst["role"])
	}
	blocks := asst["content"].([]any)
	if len(blocks) != 3 || blocks[1].(map[string]any)["type"] != "tool_use" {
		t.Fatalf("assistant blocks = %v", blocks)
	}
	if blocks[2].(map[string]any)["input"] == nil {
		t.Fatal("nil args must serialize as an object")
	}
	results := msgs[2].(map[string]any)
	if results["role"] != "user" {
		t.Fatalf("tool results role = %v", results["role"])
	}
	rb := results["content"].([]any)
	if len(rb) != 2 {
		t.Fatalf("all tool results must be in one user message: %v", rb)
	}
	if rb[1].(map[string]any)["is_error"] != true || rb[1].(map[string]any)["tool_use_id"] != "t2" {
		t.Fatalf("tool result = %v", rb[1])
	}
}

func TestBuildParamsRejects(t *testing.T) {
	tests := []struct {
		name string
		req  model.Request
	}{
		{"unknown role", model.Request{Messages: []model.Message{{Role: "robot", Content: "x"}}}},
		{"tool without results", model.Request{Messages: []model.Message{{Role: "tool"}}}},
		{"empty assistant", model.Request{Messages: []model.Message{{Role: "assistant"}}}},
		{"nameless tool", model.Request{Tools: []model.ToolManifest{{}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildParams(tt.req); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestBuildToolRequiredFromStrings(t *testing.T) {
	params, err := BuildParams(model.Request{Tools: []model.ToolManifest{{
		Name:        "t",
		InputSchema: map[string]any{"properties": map[string]any{"a": map[string]any{"type": "string"}}, "required": []string{"a"}, "$schema": "x"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(params.Tools[0])
	var wire map[string]any
	_ = json.Unmarshal(b, &wire)
	schema := wire["input_schema"].(map[string]any)
	if req := schema["required"].([]any); len(req) != 1 || req[0] != "a" {
		t.Fatalf("required = %v", schema["required"])
	}
	if schema["$schema"] != "x" || schema["additionalProperties"] != false {
		t.Fatalf("schema = %v", schema)
	}
}

func TestMapResponseUnparsableInput(t *testing.T) {
	srv := newServer(t, 200, `{"id":"m","type":"message","role":"assistant","model":"x",
	 "content":[{"type":"tool_use","id":"t","name":"n","input":{}}],"stop_reason":"tool_use","usage":{"input_tokens":1,"output_tokens":1}}`, nil)
	defer srv.Close()
	resp, err := newProvider(srv.URL).Complete(context.Background(), model.Request{Messages: []model.Message{{Role: "user", Content: "x"}}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ToolCalls[0].Args == nil || len(resp.ToolCalls[0].Args) != 0 {
		t.Fatalf("args = %v", resp.ToolCalls[0].Args)
	}
}
