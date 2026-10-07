package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/url"
	"syscall"
	"testing"

	"github.com/joeyvictorino/assay/internal/model"
)

type stub struct{ name string }

func (s stub) Name() string { return s.name }
func (s stub) Complete(context.Context, model.Request) (model.Response, error) {
	return model.Response{}, nil
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	r.Register("b", stub{"b"})
	r.Register("a", stub{"a"})
	if got := r.Names(); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("Names = %v", got)
	}
	p, err := r.Get("a")
	if err != nil || p.Name() != "a" {
		t.Fatalf("Get(a) = %v, %v", p, err)
	}
	if _, err := r.Get("zzz"); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("Get(zzz) err = %v, want ErrUnknownProvider", err)
	}
}

func TestComputeCost(t *testing.T) {
	sonnet := Cost{InputUSD: 2, OutputUSD: 10, CacheReadUSD: 0.20}
	tests := []struct {
		name  string
		usage model.Usage
		cost  Cost
		want  float64
	}{
		{"zero", model.Usage{}, sonnet, 0},
		{"1M input", model.Usage{InputTokens: 1_000_000}, sonnet, 2},
		{"1M output", model.Usage{OutputTokens: 1_000_000}, sonnet, 10},
		{"1M cache read", model.Usage{CacheReadTokens: 1_000_000}, sonnet, 0.20},
		{"cache write billed as input", model.Usage{CacheWriteTokens: 500_000}, sonnet, 1},
		{"mixed", model.Usage{InputTokens: 1000, OutputTokens: 500, CacheReadTokens: 10_000}, sonnet, 0.002 + 0.005 + 0.002},
		{"free local model", model.Usage{InputTokens: 1_000_000, OutputTokens: 1_000_000}, Cost{}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeCost(tt.usage, tt.cost)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("ComputeCost = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCostsLookup(t *testing.T) {
	c := Costs{"m": {InputUSD: 1}}
	if _, err := c.Lookup("m"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Lookup("x"); !errors.Is(err, ErrMissingCost) {
		t.Fatalf("err = %v", err)
	}
}

func TestMicroUSD(t *testing.T) {
	if got := MicroUSD(1.5); got != 1_500_000 {
		t.Fatalf("MicroUSD = %d", got)
	}
	if got := MicroUSD(0.0000015); got != 2 {
		t.Fatalf("MicroUSD rounding = %d", got)
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"429", &StatusError{Code: 429}, true},
		{"500", &StatusError{Code: 500}, true},
		{"502", &StatusError{Code: 502}, true},
		{"503", &StatusError{Code: 503}, true},
		{"529 overloaded", &StatusError{Code: 529}, true},
		{"408", &StatusError{Code: 408}, true},
		{"400", &StatusError{Code: 400}, false},
		{"401", &StatusError{Code: 401}, false},
		{"403", &StatusError{Code: 403}, false},
		{"404", &StatusError{Code: 404}, false},
		{"422", &StatusError{Code: 422}, false},
		{"wrapped 429", fmt.Errorf("outer: %w", &StatusError{Code: 429}), true},
		{"wrapped 400", fmt.Errorf("outer: %w", &StatusError{Code: 400}), false},
		{"deadline", context.DeadlineExceeded, true},
		{"canceled", context.Canceled, false},
		{"unexpected eof", io.ErrUnexpectedEOF, true},
		{"econnrefused", syscall.ECONNREFUSED, true},
		{"econnreset", &net.OpError{Op: "read", Err: syscall.ECONNRESET}, true},
		{"net timeout", timeoutErr{}, true},
		{"url error", &url.Error{Op: "Post", URL: "http://x", Err: errors.New("dial")}, true},
		{"plain", errors.New("boom"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Classify(tt.err); got != tt.want {
				t.Fatalf("Classify(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestStatusErrorString(t *testing.T) {
	e := &StatusError{Provider: "p", Code: 429, RequestID: "req_1", Err: errors.New("slow down")}
	want := "p: http 429 (request-id req_1): slow down"
	if e.Error() != want {
		t.Fatalf("Error() = %q", e.Error())
	}
	if errors.Unwrap(e) == nil {
		t.Fatal("Unwrap returned nil")
	}
}
