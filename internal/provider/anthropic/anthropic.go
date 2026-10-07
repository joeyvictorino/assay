// Package anthropic adapts the official Anthropic Go SDK to model.Provider.
//
// System prompt and tools are sent first with a cache breakpoint on the
// last system block so repeated turns hit the prompt cache. Thinking is left
// unset (adaptive by default on the current models); tool_choice is auto.
// Refusals are returned as Declined responses, never as errors and never
// retried here.
package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/joeyvictorino/assay/internal/model"
	"github.com/joeyvictorino/assay/internal/provider"
)

// DefaultModel is the owner-approved default model id.
const DefaultModel = "claude-sonnet-5-5"

// DefaultMaxTokens is used when a request leaves MaxTokens at zero.
const DefaultMaxTokens = 4096

// Config configures the provider.
type Config struct {
	// Name is the registry name (default "anthropic").
	Name string
	// APIKey overrides ANTHROPIC_API_KEY when non-empty.
	APIKey string
	// BaseURL overrides the API endpoint (tests point it at httptest).
	BaseURL string
	// MaxRetries is passed to the SDK. The router owns failover, so the
	// default is 0: the provider returns errors as they happen.
	MaxRetries int
	// Timeout bounds one request; zero means the SDK default.
	Timeout time.Duration
	// HTTPClient overrides the transport.
	HTTPClient *http.Client
}

// Provider is the Anthropic backend.
type Provider struct {
	name   string
	client sdk.Client
}

// New builds a Provider from cfg.
func New(cfg Config) *Provider {
	opts := []option.RequestOption{option.WithMaxRetries(cfg.MaxRetries)}
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, option.WithRequestTimeout(cfg.Timeout))
	}
	if cfg.HTTPClient != nil {
		opts = append(opts, option.WithHTTPClient(cfg.HTTPClient))
	}
	name := cfg.Name
	if name == "" {
		name = "anthropic"
	}
	return &Provider{name: name, client: sdk.NewClient(opts...)}
}

// Name implements model.Provider.
func (p *Provider) Name() string { return p.name }

// Complete implements model.Provider.
func (p *Provider) Complete(ctx context.Context, req model.Request) (model.Response, error) {
	params, err := BuildParams(req)
	if err != nil {
		return model.Response{}, err
	}
	var raw *http.Response
	start := time.Now()
	resp, err := p.client.Messages.New(ctx, params, option.WithResponseInto(&raw))
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return model.Response{}, p.wrapErr(err)
	}
	out := MapResponse(resp)
	out.Usage.LatencyMS = latency
	if raw != nil {
		if id := raw.Header.Get("request-id"); id != "" {
			out.RequestID = id
		}
	}
	return out, nil
}

func (p *Provider) wrapErr(err error) error {
	var apiErr *sdk.Error
	if errors.As(err, &apiErr) {
		return &provider.StatusError{Provider: p.name, Code: apiErr.StatusCode, RequestID: apiErr.RequestID, Err: err}
	}
	return fmt.Errorf("%s: %w", p.name, err)
}

// BuildParams maps a provider-neutral request to SDK params.
func BuildParams(req model.Request) (sdk.MessageNewParams, error) {
	modelID := req.Model
	if modelID == "" {
		modelID = DefaultModel
	}
	maxTokens := int64(req.MaxTokens)
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	params := sdk.MessageNewParams{Model: sdk.Model(modelID), MaxTokens: maxTokens}

	system := req.System
	for _, m := range req.Messages {
		if m.Role == "system" && m.Content != "" {
			if system != "" {
				system += "\n\n"
			}
			system += m.Content
		}
	}
	if system != "" {
		params.System = []sdk.TextBlockParam{{Text: system, CacheControl: sdk.NewCacheControlEphemeralParam()}}
	}
	for _, t := range req.Tools {
		tool, err := buildTool(t)
		if err != nil {
			return sdk.MessageNewParams{}, err
		}
		params.Tools = append(params.Tools, sdk.ToolUnionParam{OfTool: &tool})
	}
	msgs, err := buildMessages(req.Messages)
	if err != nil {
		return sdk.MessageNewParams{}, err
	}
	params.Messages = msgs
	return params, nil
}

func buildTool(t model.ToolManifest) (sdk.ToolParam, error) {
	if t.Name == "" {
		return sdk.ToolParam{}, errors.New("anthropic: tool manifest without name")
	}
	schema := sdk.ToolInputSchemaParam{ExtraFields: map[string]any{"additionalProperties": false}}
	if props, ok := t.InputSchema["properties"]; ok {
		schema.Properties = props
	} else {
		schema.Properties = map[string]any{}
	}
	schema.Required = stringSlice(t.InputSchema["required"])
	if schema.Required == nil {
		schema.Required = []string{}
	}
	for k, v := range t.InputSchema {
		switch k {
		case "type", "properties", "required", "additionalProperties":
			continue
		}
		schema.ExtraFields[k] = v
	}
	tool := sdk.ToolParam{Name: t.Name, InputSchema: schema, Strict: sdk.Bool(true)}
	if t.Description != "" {
		tool.Description = sdk.String(t.Description)
	}
	return tool, nil
}

func stringSlice(v any) []string {
	switch s := v.(type) {
	case []string:
		return append([]string(nil), s...)
	case []any:
		out := make([]string, 0, len(s))
		for _, e := range s {
			if str, ok := e.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}

func buildMessages(msgs []model.Message) ([]sdk.MessageParam, error) {
	var out []sdk.MessageParam
	for i, m := range msgs {
		switch m.Role {
		case "system":
			continue // folded into System
		case "user":
			var blocks []sdk.ContentBlockParamUnion
			for _, r := range m.ToolResults {
				blocks = append(blocks, sdk.NewToolResultBlock(r.ToolCallID, r.Content, r.IsError))
			}
			if m.Content != "" || len(blocks) == 0 {
				blocks = append(blocks, sdk.NewTextBlock(m.Content))
			}
			out = append(out, sdk.NewUserMessage(blocks...))
		case "tool":
			var blocks []sdk.ContentBlockParamUnion
			for _, r := range m.ToolResults {
				blocks = append(blocks, sdk.NewToolResultBlock(r.ToolCallID, r.Content, r.IsError))
			}
			if len(blocks) == 0 {
				return nil, fmt.Errorf("anthropic: message %d has role tool but no tool results", i)
			}
			out = append(out, sdk.NewUserMessage(blocks...))
		case "assistant":
			var blocks []sdk.ContentBlockParamUnion
			if m.Content != "" {
				blocks = append(blocks, sdk.NewTextBlock(m.Content))
			}
			for _, c := range m.ToolCalls {
				args := c.Args
				if args == nil {
					args = map[string]any{}
				}
				blocks = append(blocks, sdk.NewToolUseBlock(c.ID, args, c.Name))
			}
			if len(blocks) == 0 {
				return nil, fmt.Errorf("anthropic: message %d is an empty assistant turn", i)
			}
			out = append(out, sdk.NewAssistantMessage(blocks...))
		default:
			return nil, fmt.Errorf("anthropic: message %d has unknown role %q", i, m.Role)
		}
	}
	return out, nil
}

// MapResponse converts an SDK message to the neutral response. Refusals
// are detected before content is read.
func MapResponse(resp *sdk.Message) model.Response {
	out := model.Response{
		StopReason: string(resp.StopReason),
		RequestID:  resp.ID,
		Usage: model.Usage{
			InputTokens:      resp.Usage.InputTokens,
			OutputTokens:     resp.Usage.OutputTokens,
			CacheReadTokens:  resp.Usage.CacheReadInputTokens,
			CacheWriteTokens: resp.Usage.CacheCreationInputTokens,
		},
	}
	if resp.StopReason == sdk.StopReasonRefusal {
		out.Declined = true
		out.StopReason = "refusal"
		return out
	}
	for _, b := range resp.Content {
		switch v := b.AsAny().(type) {
		case sdk.TextBlock:
			out.Text += v.Text
		case sdk.ToolUseBlock:
			args := map[string]any{}
			if raw := v.JSON.Input.Raw(); raw != "" {
				if err := json.Unmarshal([]byte(raw), &args); err != nil {
					args = map[string]any{"_unparsed": raw}
				}
			}
			out.ToolCalls = append(out.ToolCalls, model.ToolCall{ID: v.ID, Name: v.Name, Args: args})
		}
	}
	return out
}
