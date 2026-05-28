package refine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"time"
)

// HTTPStatusError is returned by the call* functions when the upstream API
// responds with a non-200 status, so retry logic can distinguish transient
// failures (5xx, 429) from permanent ones (4xx).
type HTTPStatusError struct {
	Status int
	Body   string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body)
}

// isRetryable reports whether an error from an LLM API call is worth retrying.
// Returns true for 5xx, 429 Too Many Requests, and transient network errors.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	var hse *HTTPStatusError
	if errors.As(err, &hse) {
		return hse.Status == 429 || (hse.Status >= 500 && hse.Status <= 599)
	}
	// Context cancellation is never retryable.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}
	return false
}

// withRetry invokes fn up to maxAttempts times with exponential backoff plus
// ±50% jitter, returning the first successful result or the last error.
// Backoff: 1s, 2s, 4s (×jitter). Aborts immediately on context cancellation
// or non-retryable errors.
func withRetry(ctx context.Context, label string, fn func() (string, error)) (string, error) {
	const maxAttempts = 3
	const baseDelay = time.Second

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := fn()
		if err == nil {
			if attempt > 1 {
				slog.Info("RefinementEngine: succeeded after retry", "label", label, "attempt", attempt)
			}
			return result, nil
		}
		lastErr = err

		if attempt == maxAttempts || !isRetryable(err) {
			break
		}

		// Exponential backoff with 0.5x..1.5x jitter.
		delay := baseDelay << (attempt - 1)
		jitter := time.Duration((rand.Float64() + 0.5) * float64(delay))
		slog.Warn("RefinementEngine: retryable error, backing off",
			"label", label, "attempt", attempt, "delay", jitter, "error", err)

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(jitter):
		}
	}
	return "", lastErr
}

// callLLM dispatches to the appropriate provider call function and wraps it
// with retry logic.
func callLLM(ctx context.Context, provider, key, system, user string) (string, error) {
	return withRetry(ctx, "llm-call-"+provider, func() (string, error) {
		switch provider {
		case "openai":
			return callOpenAICompat(ctx, key, system, user)
		case "deepseek":
			return callDeepSeek(ctx, key, system, user)
		default:
			return callGemini(ctx, key, system, user)
		}
	})
}
