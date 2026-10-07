package openaicompat

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
	path string
	hdr  http.Header
	body map[string]any
}

func newServer(t *testing.T, status int, body string, cap *capture) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cap != nil {
			b, _ := io.ReadAll(r.Body)
			cap.mu.Lock()
			cap.path = r.URL.Path
			cap.hdr = r.Header.Clone()
			_ = json.Unmarshal(b, &cap.body)
			cap.mu.Unlock()
		}
		w.Header().Set("x-request-id", "oai_req_1")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

var tools = []model.ToolManifest{{
	Name:        "http_get",
	Description: "fetch",
	InputSchema: map[string]any{
		"type":       "object",
		"properties": map[string]any{"url": map[string]any{"type": "string"}},
		"required":   []any{"url"},
	},
}}

func TestCompleteText(t *testing.T) {
	cap := &capture{}
	srv := newServer(t, 200, `{"id":"chatcmpl-1","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"hello"}}],
	 "usage":{"prompt_tokens":100,"completion_tokens":7,"prompt_tokens_details":{"cached_tokens":40}}}`, cap)
	defer srv.Close()
	p := New(Config{Name: "openai", BaseURL: srv.URL + "/v1/", APIKey: "sk-test"})
	resp, err := p.Complete(context.Background(), model.Request{
		Model: "gpt-5-mini", System: "sys", MaxTokens: 50, Tools: tools,
		Messages: []model.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text != "hello" || resp.StopReason != "end_turn" || resp.Declined {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.Usage.InputTokens != 60 || resp.Usage.CacheReadTokens != 40 || resp.Usage.OutputTokens != 7 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
	if resp.RequestID != "oai_req_1" {
		t.Fatalf("request id = %q", resp.RequestID)
	}
	if p.Name() != "openai" {
		t.Fatalf("name = %s", p.Name())
	}

	cap.mu.Lock()
	defer cap.mu.Unlock()
	if cap.path != "/v1/chat/completions" {
		t.Fatalf("path = %s", cap.path)
	}
	if cap.hdr.Get("Authorization") != "Bearer sk-test" {
		t.Fatalf("auth = %q", cap.hdr.Get("Authorization"))
	}
	if cap.body["model"] != "gpt-5-mini" || cap.body["tool_choice"] != "auto" || cap.body["max_completion_tokens"] != float64(50) {
		t.Fatalf("body = %v", cap.body)
	}
	msgs := cap.body["messages"].([]any)
	if msgs[0].(map[string]any)["role"] != "system" || msgs[0].(map[string]any)["content"] != "sys" {
		t.Fatalf("system message = %v", msgs[0])
	}
	tl := cap.body["tools"].([]any)[0].(map[string]any)
	if tl["type"] != "function" {
		t.Fatalf("tool type = %v", tl["type"])
	}
	fn := tl["function"].(map[string]any)
	if fn["name"] != "http_get" || fn["strict"] != true || fn["description"] != "fetch" {
		t.Fatalf("function = %v", fn)
	}
	params := fn["parameters"].(map[string]any)
	if params["additionalProperties"] != false || params["type"] != "object" {
		t.Fatalf("parameters = %v", params)
	}
}

func TestCompleteToolCalls(t *testing.T) {
	srv := newServer(t, 200, `{"id":"chatcmpl-2","choices":[{"index":0,"finish_reason":"tool_calls","message":{"role":"assistant","content":null,
	 "tool_calls":[{"id":"call_1","type":"function","function":{"name":"http_get","arguments":"{\"url\":\"http://lab/a\"}"}},
	                {"id":"call_2","type":"function","function":{"name":"inspect_headers","arguments":""}}]}}],
	 "usage":{"prompt_tokens":10,"completion_tokens":20}}`, nil)
	defer srv.Close()
	p := New(Config{Name: "llama", BaseURL: srv.URL + "/v1"})
	resp, err := p.Complete(context.Background(), model.Request{Model: "local", Messages: []model.Message{{Role: "user", Content: "go"}}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StopReason != "tool_use" || len(resp.ToolCalls) != 2 {
		t.Fatalf("resp = %+v", resp)
	}
	if resp.ToolCalls[0].ID != "call_1" || resp.ToolCalls[0].Args["url"] != "http://lab/a" {
		t.Fatalf("call 1 = %+v", resp.ToolCalls[0])
	}
	if resp.ToolCalls[1].Args == nil || len(resp.ToolCalls[1].Args) != 0 {
		t.Fatalf("empty arguments must be an empty map: %+v", resp.ToolCalls[1])
	}
	if resp.Usage.CacheReadTokens != 0 || resp.Usage.InputTokens != 10 {
		t.Fatalf("usage = %+v", resp.Usage)
	}
}

func TestCompleteContentFilter(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"finish_reason", `{"id":"x","choices":[{"index":0,"finish_reason":"content_filter","message":{"role":"assistant","content":"partial"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`},
		{"refusal field", `{"id":"x","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":null,"refusal":"I can't help with that"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newServer(t, 200, tt.body, nil)
			defer srv.Close()
			p := New(Config{BaseURL: srv.URL + "/v1"})
			resp, err := p.Complete(context.Background(), model.Request{Messages: []model.Message{{Role: "user", Content: "x"}}})
			if err != nil {
				t.Fatal(err)
			}
			if !resp.Declined || resp.StopReason != "refusal" || resp.Text != "" {
				t.Fatalf("resp = %+v", resp)
			}
		})
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
		{"503", 503, true},
		{"400", 400, false},
		{"401", 401, false},
		{"404", 404, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newServer(t, tt.status, `{"error":{"message":"nope","type":"x"}}`, nil)
			defer srv.Close()
			p := New(Config{Name: "openai", BaseURL: srv.URL + "/v1"})
			_, err := p.Complete(context.Background(), model.Request{Messages: []model.Message{{Role: "user", Content: "x"}}})
			var se *provider.StatusError
			if !errors.As(err, &se) || se.Code != tt.status || se.RequestID != "oai_req_1" {
				t.Fatalf("err = %v", err)
			}
			if se.Err == nil || se.Err.Error() != "nope" {
				t.Fatalf("message = %v", se.Err)
			}
			if provider.Classify(err) != tt.retryable {
				t.Fatalf("Classify = %v", provider.Classify(err))
			}
		})
	}
}

func TestConnectionRefusedIsRetryable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	p := New(Config{BaseURL: url + "/v1"})
	_, err := p.Complete(context.Background(), model.Request{Messages: []model.Message{{Role: "user", Content: "x"}}})
	if err == nil || !provider.Classify(err) {
		t.Fatalf("err = %v retryable=%v", err, provider.Classify(err))
	}
}

func TestBuildRequestMessages(t *testing.T) {
	req := model.Request{
		System: "sys",
		Messages: []model.Message{
			{Role: "user", Content: "hi"},
			{Role: "assistant", ToolCalls: []model.ToolCall{{ID: "c1", Name: "http_get", Args: map[string]any{"url": "u"}}, {ID: "c2", Name: "x"}}},
			{Role: "tool", ToolResults: []model.ToolResult{{ToolCallID: "c1", Content: "ok"}, {ToolCallID: "c2", Content: "denied: policy", IsError: true}}},
		},
	}
	wr, err := BuildRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if wr.MaxTokens != DefaultMaxTokens {
		t.Fatalf("max tokens = %d", wr.MaxTokens)
	}
	if len(wr.Messages) != 5 {
		t.Fatalf("messages = %d", len(wr.Messages))
	}
	asst := wr.Messages[2]
	if asst.Role != "assistant" || asst.Content != nil || len(asst.ToolCalls) != 2 {
		t.Fatalf("assistant = %+v", asst)
	}
	if asst.ToolCalls[0].Function.Arguments != `{"url":"u"}` || asst.ToolCalls[1].Function.Arguments != `{}` {
		t.Fatalf("arguments = %q %q", asst.ToolCalls[0].Function.Arguments, asst.ToolCalls[1].Function.Arguments)
	}
	if wr.Messages[3].Role != "tool" || wr.Messages[3].ToolCallID != "c1" || *wr.Messages[3].Content != "ok" {
		t.Fatalf("tool 1 = %+v", wr.Messages[3])
	}
	if *wr.Messages[4].Content != "error: denied: policy" {
		t.Fatalf("tool 2 = %+v", *wr.Messages[4].Content)
	}
	if wr.ToolChoice != "" {
		t.Fatal("tool_choice set without tools")
	}
}

func TestBuildRequestRejects(t *testing.T) {
	tests := []struct {
		name string
		req  model.Request
	}{
		{"unknown role", model.Request{Messages: []model.Message{{Role: "robot"}}}},
		{"tool without results", model.Request{Messages: []model.Message{{Role: "tool"}}}},
		{"empty assistant", model.Request{Messages: []model.Message{{Role: "assistant"}}}},
		{"nameless tool", model.Request{Tools: []model.ToolManifest{{}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := BuildRequest(tt.req); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestMapResponseNoChoices(t *testing.T) {
	if _, err := MapResponse(wireResponse{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestMapResponseLength(t *testing.T) {
	var wr wireResponse
	_ = json.Unmarshal([]byte(`{"choices":[{"finish_reason":"length","message":{"content":"trunc"}}]}`), &wr)
	resp, err := MapResponse(wr)
	if err != nil || resp.StopReason != "max_tokens" || resp.Text != "trunc" {
		t.Fatalf("resp = %+v err = %v", resp, err)
	}
}
