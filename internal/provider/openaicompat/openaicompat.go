// Package openaicompat is a hand-rolled chat-completions client used for
// OpenAI (https://api.openai.com/v1) and for a local llama-server
// (http://127.0.0.1:8081/v1, no key). It speaks only the subset assay needs:
// system/user/assistant/tool messages, function tools with strict schemas,
// tool_choice auto, and usage with cached-token details.
package openaicompat

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/provider"
)

// DefaultMaxTokens is used when a request leaves MaxTokens at zero.
const DefaultMaxTokens = 4096

// maxErrorBody bounds how much of an error body is kept in the error.
const maxErrorBody = 512

// Config configures the provider.
type Config struct {
	// Name is the registry name, e.g. "openai" or "llama".
	Name string
	// BaseURL is the API root ending in /v1.
	BaseURL string
	// APIKey is sent as a bearer token when non-empty.
	APIKey string
	// Timeout bounds one request; zero means no client timeout (the
	// router's context deadline still applies).
	Timeout time.Duration
	// HTTPClient overrides the transport.
	HTTPClient *http.Client
}

// Provider is the chat-completions backend.
type Provider struct {
	cfg    Config
	client *http.Client
}

// New builds a Provider from cfg.
func New(cfg Config) *Provider {
	if cfg.Name == "" {
		cfg.Name = "openaicompat"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	c := cfg.HTTPClient
	if c == nil {
		c = &http.Client{Timeout: cfg.Timeout}
	}
	return &Provider{cfg: cfg, client: c}
}

// Name implements model.Provider.
func (p *Provider) Name() string { return p.cfg.Name }

// Wire types (request).
type wireMessage struct {
	Role       string         `json:"role"`
	Content    *string        `json:"content"`
	ToolCalls  []wireToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type wireToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function wireFunction `json:"function"`
}

type wireFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type wireTool struct {
	Type     string      `json:"type"`
	Function wireToolDef `json:"function"`
}

type wireToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
	Strict      bool           `json:"strict"`
}

type wireRequest struct {
	Model      string        `json:"model"`
	Messages   []wireMessage `json:"messages"`
	Tools      []wireTool    `json:"tools,omitempty"`
	ToolChoice string        `json:"tool_choice,omitempty"`
	MaxTokens  int           `json:"max_completion_tokens,omitempty"`
}

// Wire types (response).
type wireResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content   *string        `json:"content"`
			Refusal   *string        `json:"refusal"`
			ToolCalls []wireToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens        int64 `json:"prompt_tokens"`
		CompletionTokens    int64 `json:"completion_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int64 `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// BuildRequest maps the neutral request to the chat-completions body.
func BuildRequest(req model.Request) (wireRequest, error) {
	out := wireRequest{Model: req.Model, MaxTokens: req.MaxTokens}
	if out.MaxTokens <= 0 {
		out.MaxTokens = DefaultMaxTokens
	}
	if req.System != "" {
		out.Messages = append(out.Messages, wireMessage{Role: "system", Content: str(req.System)})
	}
	for i, m := range req.Messages {
		switch m.Role {
		case "system", "user":
			// The agent returns tool results on a user turn (the Anthropic
			// shape). Here they become tool messages that must directly
			// follow the assistant turn that made the calls, so they go
			// first and the user text, if any, after them. An empty user
			// message between the calls and their results is rejected by
			// strict chat templates and APIs.
			for _, r := range m.ToolResults {
				out.Messages = append(out.Messages, toolResultMessage(r))
			}
			if m.Content != "" || len(m.ToolResults) == 0 {
				out.Messages = append(out.Messages, wireMessage{Role: m.Role, Content: str(m.Content)})
			}
		case "assistant":
			wm := wireMessage{Role: "assistant"}
			if m.Content != "" {
				wm.Content = str(m.Content)
			}
			for _, c := range m.ToolCalls {
				args := c.Args
				if args == nil {
					args = map[string]any{}
				}
				b, err := json.Marshal(args)
				if err != nil {
					return wireRequest{}, fmt.Errorf("openaicompat: tool call %s args: %w", c.ID, err)
				}
				wm.ToolCalls = append(wm.ToolCalls, wireToolCall{ID: c.ID, Type: "function", Function: wireFunction{Name: c.Name, Arguments: string(b)}})
			}
			if wm.Content == nil && len(wm.ToolCalls) == 0 {
				return wireRequest{}, fmt.Errorf("openaicompat: message %d is an empty assistant turn", i)
			}
			out.Messages = append(out.Messages, wm)
		case "tool":
			if len(m.ToolResults) == 0 {
				return wireRequest{}, fmt.Errorf("openaicompat: message %d has role tool but no tool results", i)
			}
			for _, r := range m.ToolResults {
				out.Messages = append(out.Messages, toolResultMessage(r))
			}
		default:
			return wireRequest{}, fmt.Errorf("openaicompat: message %d has unknown role %q", i, m.Role)
		}
	}
	for _, t := range req.Tools {
		if t.Name == "" {
			return wireRequest{}, errors.New("openaicompat: tool manifest without name")
		}
		params := map[string]any{"type": "object", "additionalProperties": false}
		for k, v := range t.InputSchema {
			params[k] = v
		}
		params["additionalProperties"] = false
		if _, ok := params["properties"]; !ok {
			params["properties"] = map[string]any{}
		}
		if _, ok := params["required"]; !ok {
			params["required"] = []string{}
		}
		out.Tools = append(out.Tools, wireTool{Type: "function", Function: wireToolDef{
			Name: t.Name, Description: t.Description, Parameters: params, Strict: true}})
	}
	if len(out.Tools) > 0 {
		out.ToolChoice = "auto"
	}
	return out, nil
}

func toolResultMessage(r model.ToolResult) wireMessage {
	content := r.Content
	if r.IsError && !strings.HasPrefix(content, "error:") {
		content = "error: " + content
	}
	return wireMessage{Role: "tool", ToolCallID: r.ToolCallID, Content: str(content)}
}

func str(s string) *string { return &s }

// Complete implements model.Provider.
func (p *Provider) Complete(ctx context.Context, req model.Request) (model.Response, error) {
	body, err := BuildRequest(req)
	if err != nil {
		return model.Response{}, err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return model.Response{}, fmt.Errorf("%s: encode: %w", p.cfg.Name, err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return model.Response{}, fmt.Errorf("%s: %w", p.cfg.Name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}
	start := time.Now()
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return model.Response{}, fmt.Errorf("%s: %w", p.cfg.Name, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return model.Response{}, fmt.Errorf("%s: read body: %w", p.cfg.Name, err)
	}
	latency := time.Since(start).Milliseconds()
	requestID := resp.Header.Get("x-request-id")
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet := string(raw)
		if len(snippet) > maxErrorBody {
			snippet = snippet[:maxErrorBody]
		}
		var wr wireResponse
		if json.Unmarshal(raw, &wr) == nil && wr.Error != nil && wr.Error.Message != "" {
			snippet = wr.Error.Message
		}
		return model.Response{}, &provider.StatusError{Provider: p.cfg.Name, Code: resp.StatusCode, RequestID: requestID, Err: errors.New(snippet)}
	}
	var wr wireResponse
	if err := json.Unmarshal(raw, &wr); err != nil {
		return model.Response{}, fmt.Errorf("%s: decode: %w", p.cfg.Name, err)
	}
	out, err := MapResponse(wr)
	if err != nil {
		return model.Response{}, fmt.Errorf("%s: %w", p.cfg.Name, err)
	}
	out.Usage.LatencyMS = latency
	if requestID != "" {
		out.RequestID = requestID
	}
	return out, nil
}

// MapResponse converts a chat-completions body to the neutral response.
func MapResponse(wr wireResponse) (model.Response, error) {
	out := model.Response{RequestID: wr.ID}
	out.Usage.InputTokens = wr.Usage.PromptTokens
	out.Usage.OutputTokens = wr.Usage.CompletionTokens
	if d := wr.Usage.PromptTokensDetails; d != nil && d.CachedTokens > 0 {
		out.Usage.CacheReadTokens = d.CachedTokens
		if out.Usage.InputTokens >= d.CachedTokens {
			out.Usage.InputTokens -= d.CachedTokens
		}
	}
	if len(wr.Choices) == 0 {
		return model.Response{}, errors.New("response has no choices")
	}
	ch := wr.Choices[0]
	switch ch.FinishReason {
	case "content_filter":
		out.Declined = true
		out.StopReason = "refusal"
		return out, nil
	case "tool_calls", "function_call":
		out.StopReason = "tool_use"
	case "length":
		out.StopReason = "max_tokens"
	case "stop", "":
		out.StopReason = "end_turn"
	default:
		out.StopReason = ch.FinishReason
	}
	if ch.Message.Refusal != nil && *ch.Message.Refusal != "" {
		out.Declined = true
		out.StopReason = "refusal"
		return out, nil
	}
	if ch.Message.Content != nil {
		out.Text = *ch.Message.Content
	}
	for _, tc := range ch.Message.ToolCalls {
		args := map[string]any{}
		if a := strings.TrimSpace(tc.Function.Arguments); a != "" {
			if err := json.Unmarshal([]byte(a), &args); err != nil {
				args = map[string]any{"_unparsed": tc.Function.Arguments}
			}
		}
		out.ToolCalls = append(out.ToolCalls, model.ToolCall{ID: tc.ID, Name: tc.Function.Name, Args: args})
	}
	if len(out.ToolCalls) > 0 && out.StopReason == "end_turn" {
		out.StopReason = "tool_use"
	}
	return out, nil
}
