package fake

import (
	"context"
	"errors"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

func call(name string) model.ToolCall {
	return model.ToolCall{ID: "tc-" + name, Name: name, Args: map[string]any{"summary": "s"}}
}

func TestScriptOrderAndExhaustion(t *testing.T) {
	boom := errors.New("boom")
	p := New([]Step{Text("one"), Fail(boom), ToolCalls(call("a"))}, Options{})
	ctx := context.Background()
	r, err := p.Complete(ctx, model.Request{Model: "m"})
	if err != nil || r.Text != "one" || r.StopReason != "end_turn" {
		t.Fatalf("step1 = %+v, %v", r, err)
	}
	if _, err := p.Complete(ctx, model.Request{}); !errors.Is(err, boom) {
		t.Fatalf("step2 err = %v", err)
	}
	r, err = p.Complete(ctx, model.Request{})
	if err != nil || len(r.ToolCalls) != 1 || r.StopReason != "tool_use" {
		t.Fatalf("step3 = %+v, %v", r, err)
	}
	if _, err := p.Complete(ctx, model.Request{}); !errors.Is(err, ErrScriptExhausted) {
		t.Fatalf("exhausted err = %v", err)
	}
	if n := len(p.Calls()); n != 4 {
		t.Fatalf("calls = %d", n)
	}
	if p.Remaining() != 0 {
		t.Fatalf("remaining = %d", p.Remaining())
	}
	if p.Name() != "fake" {
		t.Fatalf("name = %s", p.Name())
	}
}

func TestOptions(t *testing.T) {
	tests := []struct {
		name  string
		opts  Options
		step  Step
		check func(t *testing.T, r model.Response)
	}{
		{"canary", Options{Canary: "CANARY-1"}, Text("hi"), func(t *testing.T, r model.Response) {
			if r.Text != "hiCANARY-1" {
				t.Fatalf("text = %q", r.Text)
			}
		}},
		{"refuse", Options{Refuse: true}, Text("hi"), func(t *testing.T, r model.Response) {
			if !r.Declined || r.StopReason != "refusal" || r.Text != "" || len(r.ToolCalls) != 0 {
				t.Fatalf("resp = %+v", r)
			}
		}},
		{"refusal step", Options{}, Refusal(), func(t *testing.T, r model.Response) {
			if !r.Declined || r.StopReason != "refusal" {
				t.Fatalf("resp = %+v", r)
			}
		}},
		{"duplicate ids", Options{DuplicateToolCallIDs: true}, ToolCalls(call("a"), call("b")), func(t *testing.T, r model.Response) {
			if r.ToolCalls[0].ID != r.ToolCalls[1].ID {
				t.Fatalf("ids differ: %v", r.ToolCalls)
			}
		}},
		{"missing ids", Options{MissingToolCallIDs: true}, ToolCalls(call("a")), func(t *testing.T, r model.Response) {
			if r.ToolCalls[0].ID != "" {
				t.Fatalf("id = %q", r.ToolCalls[0].ID)
			}
		}},
		{"oversize summary", Options{OversizeSummary: true}, ToolCalls(call("report_finding"), call("http_get")), func(t *testing.T, r model.Response) {
			if s := r.ToolCalls[0].Args["summary"].(string); len(s) < 280 {
				t.Fatalf("summary not oversized: %d", len(s))
			}
			if s := r.ToolCalls[1].Args["summary"].(string); s != "s" {
				t.Fatalf("other tool touched: %q", s)
			}
		}},
		{"custom name", Options{Name: "llama"}, Text("x"), func(t *testing.T, r model.Response) {}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New([]Step{tt.step}, tt.opts)
			r, err := p.Complete(context.Background(), model.Request{})
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, r)
			if tt.opts.Name != "" && p.Name() != tt.opts.Name {
				t.Fatalf("name = %s", p.Name())
			}
		})
	}
}

func TestScriptNotMutated(t *testing.T) {
	step := ToolCalls(call("report_finding"))
	p := New([]Step{step}, Options{OversizeSummary: true, MissingToolCallIDs: true})
	if _, err := p.Complete(context.Background(), model.Request{}); err != nil {
		t.Fatal(err)
	}
	if step.Response.ToolCalls[0].ID != "tc-report_finding" || step.Response.ToolCalls[0].Args["summary"] != "s" {
		t.Fatalf("script step mutated: %+v", step.Response.ToolCalls[0])
	}
}

func TestCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := New([]Step{Text("x")}, Options{})
	if _, err := p.Complete(ctx, model.Request{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	if p.Remaining() != 1 {
		t.Fatal("step consumed on canceled context")
	}
}
