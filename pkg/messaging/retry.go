package messaging

import (
	"context"
	"errors"
	"time"
)

// RetryPolicy defines a compact retry strategy suitable for relay and consumer loops.
type RetryPolicy struct {
	MaxAttempts int
	Backoff     time.Duration
	MaxBackoff  time.Duration
}

// Retry executes fn until success, context cancellation, or retry exhaustion.
func Retry(ctx context.Context, policy RetryPolicy, fn func(context.Context) error) error {
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = 1
	}
	if policy.Backoff <= 0 {
		policy.Backoff = 250 * time.Millisecond
	}
	if policy.MaxBackoff <= 0 {
		policy.MaxBackoff = 5 * time.Second
	}

	var lastErr error
	delay := policy.Backoff

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if err := fn(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if attempt == policy.MaxAttempts {
			break
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		delay *= 2
		if delay > policy.MaxBackoff {
			delay = policy.MaxBackoff
		}
	}

	if lastErr == nil {
		lastErr = errors.New("retry exhausted")
	}
	return lastErr
}
