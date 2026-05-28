package refine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsRetryable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"5xx", &HTTPStatusError{Status: 503}, true},
		{"500", &HTTPStatusError{Status: 500}, true},
		{"429", &HTTPStatusError{Status: 429}, true},
		{"400", &HTTPStatusError{Status: 400}, false},
		{"401", &HTTPStatusError{Status: 401}, false},
		{"plain error", errors.New("nope"), false},
		{"context canceled", context.Canceled, false},
		{"context deadline", context.DeadlineExceeded, false},
		{"net timeout", &timeoutErr{}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isRetryable(tc.err)
			if got != tc.want {
				t.Errorf("isRetryable(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestWithRetrySucceedsAfterTransient(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	out, err := withRetry(t.Context(), "test", func() (string, error) {
		n := calls.Add(1)
		if n < 3 {
			return "", &HTTPStatusError{Status: 503, Body: "overloaded"}
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out != "ok" {
		t.Errorf("out = %q", out)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

func TestWithRetryStopsOnPermanentError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	_, err := withRetry(t.Context(), "test", func() (string, error) {
		calls.Add(1)
		return "", &HTTPStatusError{Status: 400, Body: "bad request"}
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want 1 (no retry on 400)", got)
	}
}

func TestWithRetryExhaustsAttempts(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	// Use a short-deadline context so the test doesn't sleep through real backoffs.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := withRetry(ctx, "test", func() (string, error) {
		calls.Add(1)
		return "", &HTTPStatusError{Status: 503}
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if got := calls.Load(); got < 1 {
		t.Errorf("calls = %d, want >= 1", got)
	}
}

func TestWithRetryRespectsContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := withRetry(ctx, "test", func() (string, error) {
		return "", &HTTPStatusError{Status: 503}
	})
	if !errors.Is(err, context.Canceled) && err == nil {
		t.Errorf("err = %v, want canceled or non-nil error", err)
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

var _ net.Error = timeoutErr{}

func TestHTTPStatusErrorFormat(t *testing.T) {
	t.Parallel()
	e := &HTTPStatusError{Status: 429, Body: "rate limited"}
	want := "HTTP 429: rate limited"
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	// Confirm wrapping works.
	wrapped := fmt.Errorf("calling LLM API: %w", e)
	var extracted *HTTPStatusError
	if !errors.As(wrapped, &extracted) {
		t.Error("errors.As did not extract HTTPStatusError")
	} else if extracted.Status != 429 {
		t.Errorf("extracted status = %d", extracted.Status)
	}
}
