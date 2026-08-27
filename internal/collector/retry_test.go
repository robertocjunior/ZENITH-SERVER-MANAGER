package collector

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoWithRetrySuccessFirstTry(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
		Multiplier:     2.0,
		JitterFraction: 0,
	}

	var callCount int32
	err := DoWithRetry(context.Background(), cfg, func(ctx context.Context, attempt int) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}
}

func TestDoWithRetrySuccessAfterFailures(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:    4,
		InitialBackoff: 10 * time.Millisecond,
		MaxBackoff:     50 * time.Millisecond,
		Multiplier:     2.0,
		JitterFraction: 0,
	}

	var callCount int32
	err := DoWithRetry(context.Background(), cfg, func(ctx context.Context, attempt int) error {
		current := atomic.AddInt32(&callCount, 1)
		if current < 3 {
			return errors.New("temporary network timeout")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected nil error after retry, got %v", err)
	}
	if atomic.LoadInt32(&callCount) != 3 {
		t.Fatalf("expected 3 calls, got %d", callCount)
	}
}

func TestDoWithRetryExhaustion(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:    3,
		InitialBackoff: 5 * time.Millisecond,
		MaxBackoff:     20 * time.Millisecond,
		Multiplier:     2.0,
		JitterFraction: 0,
	}

	expectedErr := errors.New("winrm connection refused")
	var callCount int32
	err := DoWithRetry(context.Background(), cfg, func(ctx context.Context, attempt int) error {
		atomic.AddInt32(&callCount, 1)
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
	if atomic.LoadInt32(&callCount) != 3 {
		t.Fatalf("expected 3 calls before exhaustion, got %d", callCount)
	}
}

func TestDoWithRetryContextCancelled(t *testing.T) {
	cfg := RetryConfig{
		MaxAttempts:    5,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     500 * time.Millisecond,
		Multiplier:     2.0,
		JitterFraction: 0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	err := DoWithRetry(ctx, cfg, func(ctx context.Context, attempt int) error {
		return errors.New("disconnected")
	})

	if err == nil {
		t.Fatal("expected context cancelled error, got nil")
	}
}
