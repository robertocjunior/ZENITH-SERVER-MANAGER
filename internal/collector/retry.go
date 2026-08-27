package collector

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// RetryConfig defines exponential backoff parameters.
type RetryConfig struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
	JitterFraction float64
}

// DefaultRetryConfig returns a robust retry configuration for remote network calls.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
		JitterFraction: 0.1,
	}
}

// DoWithRetry executes an operation with exponential backoff and jitter until success or max attempts reached.
func DoWithRetry(ctx context.Context, cfg RetryConfig, operation func(ctx context.Context, attempt int) error) error {
	var lastErr error
	backoff := cfg.InitialBackoff

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return errors.Join(ctx.Err(), lastErr)
			}
			return ctx.Err()
		default:
		}

		err := operation(ctx, attempt)
		if err == nil {
			return nil
		}

		lastErr = err

		if attempt >= cfg.MaxAttempts {
			break
		}

		// Calculate jitter: +/- jitterFraction
		jitterRange := float64(backoff) * cfg.JitterFraction
		var jitter time.Duration
		if jitterRange > 0 {
			jitter = time.Duration((rand.Float64()*2 - 1) * jitterRange)
		}
		sleepDuration := backoff + jitter
		if sleepDuration > cfg.MaxBackoff {
			sleepDuration = cfg.MaxBackoff
		}
		if sleepDuration < 0 {
			sleepDuration = backoff
		}

		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), lastErr)
		case <-time.After(sleepDuration):
		}

		// Increase backoff
		backoff = time.Duration(float64(backoff) * cfg.Multiplier)
		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}
	}

	return lastErr
}
