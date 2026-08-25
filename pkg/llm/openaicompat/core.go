package openaicompat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	llmshared "github.com/kcaldas/genie/pkg/llm/shared"
)

// DefaultChatEndpoint is the chat-completions path relative to the base URL.
const DefaultChatEndpoint = "/chat/completions"

var (
	// ErrEmptyResponse marks a model answer with neither text nor tool
	// calls before any tool activity.
	ErrEmptyResponse = errors.New("model returned an empty response")
	// ErrToolCallNoHandler marks tool calls requested without handlers.
	ErrToolCallNoHandler = errors.New("model requested tool calls but no handlers were provided")
)

// Core bundles the transport, telemetry and turn machinery of an
// OpenAI-compatible chat-completions provider. Providers embed it and
// keep only credentials, base-URL resolution and request assembly.
type Core struct {
	llmshared.LocalClientCore
	// ChatEndpoint is the completions path appended to BaseURL.
	ChatEndpoint string
	// StreamIncludeUsage sends stream_options {"include_usage": true}
	// on streaming requests. Off by default: support varies across
	// local servers.
	StreamIncludeUsage bool
}

// NewCore builds a core for the named provider with the default
// dependency set and chat endpoint.
func NewCore(provider string, eventBus events.EventBus) Core {
	return Core{
		LocalClientCore: llmshared.NewLocalClientCore(provider, eventBus),
		ChatEndpoint:    DefaultChatEndpoint,
	}
}

func (c *Core) chatURL() string {
	endpoint := c.ChatEndpoint
	if endpoint == "" {
		endpoint = DefaultChatEndpoint
	}
	return c.BaseURL + endpoint
}

// SendChat executes one blocking chat-completions request.
func (c *Core) SendChat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	req.Stream = false
	req.StreamOptions = nil
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.chatURL()
	c.Logger.Debug(c.Provider+" request", "url", url, "body", string(payload))

	resp, err := c.PostJSON(ctx, url, payload)
	if err != nil {
		return nil, fmt.Errorf("%s chat request failed: %w", c.Provider, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading %s response: %w", c.Provider, err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s chat request failed: status %s: %s", c.Provider, resp.Status, string(body))
	}

	var response ChatResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decoding %s response: %w", c.Provider, err)
	}

	c.Logger.Debug(c.Provider+" response", "status", resp.StatusCode, "body", string(body))

	if response.Error != nil && response.Error.Message != "" {
		return nil, fmt.Errorf("%s error: %s", c.Provider, response.Error.Message)
	}

	return &response, nil
}

// SendChatStream executes one streaming chat-completions request,
// passing each decoded SSE chunk to handler.
func (c *Core) SendChatStream(ctx context.Context, req ChatRequest, handler func(*ChatStreamResponse) error) error {
	req.Stream = true
	if c.StreamIncludeUsage {
		req.StreamOptions = &StreamOptions{IncludeUsage: true}
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := c.chatURL()
	c.Logger.Debug(c.Provider+" stream request", "url", url, "body", string(payload))

	resp, err := c.PostJSON(ctx, url, payload)
	if err != nil {
		return fmt.Errorf("%s chat request failed: %w", c.Provider, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s chat request failed: status %s: %s", c.Provider, resp.Status, string(body))
	}

	return llmshared.ScanStreamLines(resp.Body, c.Provider, func(line string) error {
		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
		if line == "" || line == "[DONE]" {
			return nil
		}
		if strings.HasPrefix(line, "event:") {
			// Streams may include event metadata lines; skip non-JSON events.
			return nil
		}
		if line[0] != '{' && line[0] != '[' {
			return nil
		}

		c.Logger.Debug(c.Provider+" stream chunk", "chunk", line)

		var chunk ChatStreamResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return fmt.Errorf("decoding %s stream chunk: %w", c.Provider, err)
		}

		return handler(&chunk)
	})
}

// PublishUsage emits a TokenCountEvent for one model response. When the
// server reports a cache split, InputTokens carries only the uncached
// (cache-miss) input — consistent with the OpenAI and Anthropic
// providers — and the cached portion lands in CachedTokens /
// CacheReadInputTokens. Returns the provider-neutral token count, or
// nil for nil usage.
func (c *Core) PublishUsage(ctx context.Context, modelName string, u *Usage) *ai.TokenCount {
	if u == nil {
		return nil
	}

	tokenCount := u.TokenCount()

	cached := u.PromptCacheHitTokens
	uncached := u.PromptTokens - cached
	if uncached < 0 {
		uncached = u.PromptTokens
		cached = 0
	}

	if strings.TrimSpace(modelName) == "" {
		modelName = c.ResolveModelName("")
	}

	event := events.TokenCountEvent{
		RequestID:            ai.RequestIDFromContext(ctx),
		Provider:             c.Provider,
		Model:                modelName,
		InputTokens:          uncached,
		OutputTokens:         u.CompletionTokens,
		CachedTokens:         cached,
		CacheReadInputTokens: cached,
		TotalTokens:          tokenCount.TotalTokens,
	}
	c.EventBus.Publish(event.Topic(), event)

	return tokenCount
}
