package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
)

// StatusError is a provider HTTP failure with its status code. Backends
// wrap SDK or transport errors in it so Classify can decide retryability
// without importing any SDK. Unwrap returns the original error.
type StatusError struct {
	Provider  string
	Code      int
	RequestID string
	Err       error
}

func (e *StatusError) Error() string {
	msg := fmt.Sprintf("%s: http %d", e.Provider, e.Code)
	if e.RequestID != "" {
		msg += " (request-id " + e.RequestID + ")"
	}
	if e.Err != nil {
		msg += ": " + e.Err.Error()
	}
	return msg
}

// Unwrap exposes the wrapped error to errors.Is / errors.As.
func (e *StatusError) Unwrap() error { return e.Err }

// Classify reports whether err is a transient transport failure the router
// may fail over from. Retryable: 408, 409, 425, 429, every 5xx, timeouts,
// connection resets or refusals, unexpected EOF and context deadline
// expiry. Not retryable: 400, 401, 403, 404, 422, context cancellation and
// anything unknown. nil is not retryable.
func Classify(err error) bool {
	if err == nil {
		return false
	}
	var se *StatusError
	if errors.As(err, &se) {
		return retryableStatus(se.Code)
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	var ue *url.Error
	if errors.As(err, &ue) {
		return true
	}
	var oe *net.OpError
	if errors.As(err, &oe) {
		return true
	}
	return false
}

func retryableStatus(code int) bool {
	switch {
	case code >= 500:
		return true
	case code == http.StatusTooManyRequests, code == http.StatusRequestTimeout,
		code == http.StatusConflict, code == http.StatusTooEarly:
		return true
	}
	return false
}
