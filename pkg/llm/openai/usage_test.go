package openai

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureTokenEvents(t *testing.T) (*Client, func() events.TokenCountEvent) {
	t.Helper()
	bus := events.NewEventBus()
	var mu sync.Mutex
	var got []events.TokenCountEvent
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(e interface{}) {
		if tc, ok := e.(events.TokenCountEvent); ok {
			mu.Lock()
			got = append(got, tc)
			mu.Unlock()
		}
	})
	client := &Client{config: config.NewConfigManager(), eventBus: bus}
	return client, func() events.TokenCountEvent {
		require.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(got) == 1 }, time.Second, 10*time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		return got[0]
	}
}

// GPT-5.6-class models report cache_write_tokens inside the token details;
// the pinned SDK predates the field, so it is read from the raw JSON. Written
// tokens are billed at 1.25x and are a subset of input_tokens, so they leave
// InputTokens and ride as CacheCreationInputTokens, matching Anthropic's
// split and the rates seed that prices it.
func TestPublishResponsesUsage_ReportsCacheWriteTokens(t *testing.T) {
	var usage responses.ResponseUsage
	require.NoError(t, json.Unmarshal([]byte(`{"input_tokens":1000,"output_tokens":50,"total_tokens":1050,
		"input_tokens_details":{"cached_tokens":600,"cache_write_tokens":250},
		"output_tokens_details":{"reasoning_tokens":0}}`), &usage))
	client, next := captureTokenEvents(t)

	count := client.publishResponsesUsage(context.Background(), "gpt-5.6-luna", usage)
	event := next()

	assert.Equal(t, int32(150), event.InputTokens, "uncached, unwritten input")
	assert.Equal(t, int32(600), event.CachedTokens)
	assert.Equal(t, int32(600), event.CacheReadInputTokens)
	assert.Equal(t, int32(250), event.CacheCreationInputTokens)
	assert.Equal(t, int32(50), event.OutputTokens)
	assert.Equal(t, int32(1050), event.TotalTokens)
	require.NotNil(t, count)
	assert.Equal(t, int32(1000), count.InputTokens, "the returned count stays the physical prompt size")
}

func TestPublishResponsesUsage_NoCacheWriteFieldMeansZero(t *testing.T) {
	var usage responses.ResponseUsage
	require.NoError(t, json.Unmarshal([]byte(`{"input_tokens":1000,"output_tokens":50,"total_tokens":1050,
		"input_tokens_details":{"cached_tokens":600},"output_tokens_details":{"reasoning_tokens":0}}`), &usage))
	client, next := captureTokenEvents(t)

	client.publishResponsesUsage(context.Background(), "gpt-5.4-mini", usage)
	event := next()

	assert.Equal(t, int32(400), event.InputTokens)
	assert.Equal(t, int32(0), event.CacheCreationInputTokens)
}

func TestPublishUsage_ChatCompletionsReportsCacheWriteTokens(t *testing.T) {
	var usage openai.CompletionUsage
	require.NoError(t, json.Unmarshal([]byte(`{"prompt_tokens":1000,"completion_tokens":50,"total_tokens":1050,
		"prompt_tokens_details":{"cached_tokens":600,"cache_write_tokens":250}}`), &usage))
	client, next := captureTokenEvents(t)

	client.publishUsage(context.Background(), "gpt-5.6-luna", usage)
	event := next()

	assert.Equal(t, int32(150), event.InputTokens)
	assert.Equal(t, int32(250), event.CacheCreationInputTokens)
}
