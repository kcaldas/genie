package ai

import (
	"context"
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

// RetryMiddleware wraps an AI Gen implementation to add retry logic
type RetryMiddleware struct {
	underlying Gen
	maxRetries int
	initialBackoff time.Duration
}

// NewRetryMiddleware creates a new RetryMiddleware
func NewRetryMiddleware(underlying Gen, config RetryConfig) *RetryMiddleware {
	return &RetryMiddleware{
		underlying: underlying,
		maxRetries: config.MaxRetries,
		initialBackoff: config.InitialBackoff,
	}
}

// GenerateContent implements the Gen interface with retry logic
func (r *RetryMiddleware) GenerateContent(ctx context.Context, p Prompt, debug bool, args ...string) (string, error) {
	var (
		response string
		err      error
	)
	
	backoff := r.initialBackoff
	for i := 0; i < r.maxRetries; i++ {
		response, err = r.underlying.GenerateContent(ctx, p, debug, args...)
		if err == nil {
			return response, nil
		}

		log.Printf("Attempt %d failed: %v. Retrying in %v...", i+1, err, backoff)
		time.Sleep(backoff)
		backoff *= 2 // Exponential backoff
	}

	return "", fmt.Errorf("failed to generate content after %d retries: %w", r.maxRetries, err)
}

// GenerateContentAttr implements the Gen interface with retry logic
func (r *RetryMiddleware) GenerateContentAttr(ctx context.Context, p Prompt, debug bool, attrs []Attr) (string, error) {
	var (
		response string
		err      error
	)

	backoff := r.initialBackoff
	for i := 0; i < r.maxRetries; i++ {
		response, err = r.underlying.GenerateContentAttr(ctx, p, debug, attrs)
		if err == nil {
			return response, nil
		}

		log.Printf("Attempt %d failed: %v. Retrying in %v...", i+1, err, backoff)
		time.Sleep(backoff)
		backoff *= 2 // Exponential backoff
	}

	return "", fmt.Errorf("failed to generate content with attributes after %d retries: %w", r.maxRetries, err)
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

// GetRetryConfigFromEnv creates retry config from environment variables
func GetRetryConfigFromEnv(configManager config.Manager) RetryConfig {
	return RetryConfig{
		Enabled:        configManager.GetBoolWithDefault("GENIE_RETRY_LLM_ENABLED", true),
		MaxRetries:     configManager.GetIntWithDefault("GENIE_RETRY_LLM_MAX_RETRIES", 3),
		InitialBackoff: configManager.GetDurationWithDefault("GENIE_RETRY_LLM_INITIAL_BACKOFF", 1*time.Second),
	}
}
