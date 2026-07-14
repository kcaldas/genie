package ai

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/kcaldas/genie/pkg/config"
)

// RetryConfig configures the retry middleware
type RetryConfig struct {
	Enabled        bool
	MaxRetries     int
	InitialBackoff time.Duration
}

// nonRetryableError marks an error as permanent: retrying can never
// succeed (missing API keys, invalid requests, exhausted quotas that
// require human action).
type nonRetryableError struct {
	err error
}

func (e *nonRetryableError) Error() string { return e.err.Error() }
func (e *nonRetryableError) Unwrap() error { return e.err }

// NonRetryable wraps an error so retry middleware fails fast instead
// of repeating attempts that cannot succeed.
func NonRetryable(err error) error {
	if err == nil {
		return nil
	}
	return &nonRetryableError{err: err}
}

// IsRetryable reports whether an attempt that failed with err is worth
// repeating. Cancellations are user decisions and permanent errors
// cannot be fixed by trying again.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var permanent *nonRetryableError
	return !errors.As(err, &permanent)
}

// RetryMiddleware wraps an AI Gen implementation to add retry logic
type RetryMiddleware struct {
	underlying     Gen
	maxRetries     int
	initialBackoff time.Duration
}

// NewRetryMiddleware creates a new RetryMiddleware
func NewRetryMiddleware(underlying Gen, config RetryConfig) *RetryMiddleware {
	return &RetryMiddleware{
		underlying:     underlying,
		maxRetries:     config.MaxRetries,
		initialBackoff: config.InitialBackoff,
	}
}

// GenerateContent implements the Gen interface with retry logic
func (r *RetryMiddleware) GenerateContent(ctx context.Context, p Prompt, debug bool, args ...string) (string, error) {
	return retryWithBackoff(ctx, r.maxRetries, r.initialBackoff, "generate content", func() (string, error) {
		return r.underlying.GenerateContent(ctx, p, debug, args...)
	})
}

// GenerateContentAttr implements the Gen interface with retry logic
func (r *RetryMiddleware) GenerateContentAttr(ctx context.Context, p Prompt, debug bool, attrs []Attr) (string, error) {
	return retryWithBackoff(ctx, r.maxRetries, r.initialBackoff, "generate content", func() (string, error) {
		return r.underlying.GenerateContentAttr(ctx, p, debug, attrs)
	})
}

func (r *RetryMiddleware) GenerateContentStream(ctx context.Context, p Prompt, debug bool, args ...string) (Stream, error) {
	return retryWithBackoff(ctx, r.maxRetries, r.initialBackoff, "open stream", func() (Stream, error) {
		return r.underlying.GenerateContentStream(ctx, p, debug, args...)
	})
}

func (r *RetryMiddleware) GenerateContentAttrStream(ctx context.Context, p Prompt, debug bool, attrs []Attr) (Stream, error) {
	return retryWithBackoff(ctx, r.maxRetries, r.initialBackoff, "open stream", func() (Stream, error) {
		return r.underlying.GenerateContentAttrStream(ctx, p, debug, attrs)
	})
}

// CountTokens delegates to the underlying LLM client
func (r *RetryMiddleware) CountTokens(ctx context.Context, p Prompt, debug bool, args ...string) (*TokenCount, error) {
	return r.underlying.CountTokens(ctx, p, debug, args...)
}

// CountTokensAttr delegates to the underlying LLM client
func (r *RetryMiddleware) CountTokensAttr(ctx context.Context, p Prompt, debug bool, attrs []Attr) (*TokenCount, error) {
	return r.underlying.CountTokensAttr(ctx, p, debug, attrs)
}

// GetStatus delegates to the underlying LLM client
func (r *RetryMiddleware) GetStatus() *Status {
	return r.underlying.GetStatus()
}

// retryWithBackoff runs attempt up to maxRetries times with exponential
// backoff. It fails fast on non-retryable errors and honors context
// cancellation both between attempts and during backoff sleeps.
func retryWithBackoff[T any](ctx context.Context, maxRetries int, initialBackoff time.Duration, what string, attempt func() (T, error)) (T, error) {
	var (
		result T
		err    error
		zero   T
	)

	backoff := initialBackoff
	for i := 0; i < maxRetries; i++ {
		result, err = attempt()
		if err == nil {
			return result, nil
		}
		if !IsRetryable(err) {
			return zero, err
		}

		log.Printf("Attempt %d to %s failed: %v. Retrying in %v...", i+1, what, err, backoff)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return zero, fmt.Errorf("aborted while waiting to retry: %w", ctx.Err())
		}
		backoff *= 2 // Exponential backoff
	}

	return zero, fmt.Errorf("failed to %s after %d retries: %w", what, maxRetries, err)
}

// GetRetryConfigFromEnv creates retry config from environment variables
func GetRetryConfigFromEnv(configManager config.Manager) RetryConfig {
	return RetryConfig{
		Enabled:        configManager.GetBoolWithDefault("GENIE_RETRY_LLM_ENABLED", true),
		MaxRetries:     configManager.GetIntWithDefault("GENIE_RETRY_LLM_MAX_RETRIES", 3),
		InitialBackoff: configManager.GetDurationWithDefault("GENIE_RETRY_LLM_INITIAL_BACKOFF", 1*time.Second),
	}
}
