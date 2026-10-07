// Package fake is a scripted model.Provider for tests and CI pull-request
// runs. It is deterministic: the same Script produces the same responses in
// the same order, and it never touches the network.
package fake

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/joeyvictorino/assay/internal/model"
)

// ErrScriptExhausted is returned once every Step has been consumed.
var ErrScriptExhausted = errors.New("fake: script exhausted")

// Step is one scripted turn: either a canned response or an error.
type Step struct {
	Response model.Response
	Err      error
}

// Options tune the fault injection applied to every scripted response.
type Options struct {
	// Canary, when set, is appended to every response's Text. Tests assert
	// it never leaks into findings, audit records or results.
	Canary string
	// Refuse makes every response a safety refusal (Declined=true,
	// StopReason refusal, no text, no tool calls).
	Refuse bool
	// DuplicateToolCallIDs gives every tool call in a response the id of
	// the first call.
	DuplicateToolCallIDs bool
	// MissingToolCallIDs blanks every tool call id.
	MissingToolCallIDs bool
	// OversizeSummary pads the "summary" argument of every report_finding
	// call past the 280-rune bound.
	OversizeSummary bool
	// Name overrides the provider name (default "fake").
	Name string
}

// Provider replays a Script.
type Provider struct {
	mu     sync.Mutex
	script []Step
	next   int
	opts   Options
	calls  []model.Request
}

// New returns a provider that replays script in order.
func New(script []Step, opts Options) *Provider {
	return &Provider{script: script, opts: opts}
}

// Text builds a Step holding a plain end_turn text response.
func Text(text string) Step {
	return Step{Response: model.Response{Text: text, StopReason: "end_turn",
		Usage: model.Usage{InputTokens: 100, OutputTokens: 20}}}
}

// ToolCalls builds a Step holding a tool_use response.
func ToolCalls(calls ...model.ToolCall) Step {
	return Step{Response: model.Response{ToolCalls: calls, StopReason: "tool_use",
		Usage: model.Usage{InputTokens: 120, OutputTokens: 40}}}
}

// Refusal builds a Step holding a provider safety refusal.
func Refusal() Step {
	return Step{Response: model.Response{Declined: true, StopReason: "refusal",
		Usage: model.Usage{InputTokens: 50, OutputTokens: 0}}}
}

// Fail builds a Step returning err.
func Fail(err error) Step { return Step{Err: err} }

// Name implements model.Provider.
func (p *Provider) Name() string {
	if p.opts.Name != "" {
		return p.opts.Name
	}
	return "fake"
}

// Calls returns a copy of every request seen so far.
func (p *Provider) Calls() []model.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]model.Request(nil), p.calls...)
}

// Remaining reports how many steps are left.
func (p *Provider) Remaining() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.script) - p.next
}

// Complete implements model.Provider.
func (p *Provider) Complete(ctx context.Context, req model.Request) (model.Response, error) {
	if err := ctx.Err(); err != nil {
		return model.Response{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, req)
	if p.next >= len(p.script) {
		return model.Response{}, ErrScriptExhausted
	}
	step := p.script[p.next]
	p.next++
	if step.Err != nil {
		return model.Response{}, step.Err
	}
	return p.apply(step.Response), nil
}

func (p *Provider) apply(in model.Response) model.Response {
	out := in
	out.ToolCalls = make([]model.ToolCall, len(in.ToolCalls))
	for i, c := range in.ToolCalls {
		cc := c
		cc.Args = make(map[string]any, len(c.Args))
		for k, v := range c.Args {
			cc.Args[k] = v
		}
		out.ToolCalls[i] = cc
	}
	if p.opts.Refuse {
		return model.Response{Declined: true, StopReason: "refusal", Usage: in.Usage, RequestID: in.RequestID}
	}
	if p.opts.Canary != "" {
		out.Text = out.Text + p.opts.Canary
	}
	if p.opts.DuplicateToolCallIDs && len(out.ToolCalls) > 1 {
		for i := range out.ToolCalls {
			out.ToolCalls[i].ID = out.ToolCalls[0].ID
		}
	}
	if p.opts.MissingToolCallIDs {
		for i := range out.ToolCalls {
			out.ToolCalls[i].ID = ""
		}
	}
	if p.opts.OversizeSummary {
		for i := range out.ToolCalls {
			if out.ToolCalls[i].Name != "report_finding" {
				continue
			}
			s, _ := out.ToolCalls[i].Args["summary"].(string)
			out.ToolCalls[i].Args["summary"] = s + strings.Repeat("x", 600)
		}
	}
	if out.RequestID == "" {
		out.RequestID = "fake-req"
	}
	return out
}
