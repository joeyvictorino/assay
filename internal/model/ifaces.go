package model

import "context"

// Auditor appends one record to the append-only, encrypted audit log and
// returns its sequence number. Implemented by internal/audit (S1).
type Auditor interface {
	Record(ctx context.Context, rec AuditRecord) (seq uint64, err error)
}

// PolicyEvaluator decides whether an agent may execute a tool now.
// Implemented by internal/policy (S1). Deny wins; default deny.
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, req PolicyRequest) Decision
}

// ManifestVerifier checks a tool manifest signature against the trust store.
// Implemented by internal/toolsig (S1). Returns a reason code on failure.
type ManifestVerifier interface {
	Verify(m ToolManifest) (keyID string, reason string, err error)
}

// Gate is the authorization gate: a target URL is allowed only if the signed
// scope document permits it. Implemented by internal/scope (S3). Fails closed.
type Gate interface {
	Allow(ctx context.Context, targetURL string) Decision
}

// TranscriptSink receives model transcripts. The only production
// implementation is a sink that discards everything (zero data retention).
// Implemented by internal/zdr (S1).
type TranscriptSink interface {
	Write(ctx context.Context, runID, agent string, body []byte) error
}

// Request is a provider-neutral model request.
type Request struct {
	Model     string         `json:"model"`
	System    string         `json:"system"`
	Messages  []Message      `json:"messages"`
	Tools     []ToolManifest `json:"tools,omitempty"`
	MaxTokens int            `json:"max_tokens"`
	TaskID    string         `json:"task_id"`
	Agent     string         `json:"agent"`
}

// Message is one turn. Content is text; ToolCalls/ToolResults carry the
// structured parts so providers can map them to their wire formats.
type Message struct {
	Role        string       `json:"role"` // system | user | assistant | tool
	Content     string       `json:"content,omitempty"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
}

// ToolCall is a model-emitted call.
type ToolCall struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// ToolResult is what the harness feeds back.
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
	IsError    bool   `json:"is_error"`
}

// Response is a provider-neutral model response.
type Response struct {
	Text       string     `json:"text"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	StopReason string     `json:"stop_reason"` // end_turn | tool_use | max_tokens | refusal | error
	Declined   bool       `json:"declined"`    // provider safety refusal; never retried or rephrased
	Usage      Usage      `json:"usage"`
	RequestID  string     `json:"request_id,omitempty"`
}

// Provider is a pluggable model backend. Implemented by internal/provider/*
// (S2). Transport failures return err; refusals return Declined=true, nil.
type Provider interface {
	Name() string
	Complete(ctx context.Context, req Request) (Response, error)
}
