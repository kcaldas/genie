package genai

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"
)

// Thinking tokens bill at the output rate, but Gemini reports them apart
// from candidatesTokenCount; output must include them or usage undercounts.
func TestPublishUsageMetadataCountsThinkingAsOutput(t *testing.T) {
	bus := events.NewEventBus()
	got := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(e interface{}) {
		if tc, ok := e.(events.TokenCountEvent); ok {
			got <- tc
		}
	})
	client := &Client{Config: config.NewConfigManager(), EventBus: bus}

	count := client.publishUsageMetadata(context.Background(), "gemini-3.5-flash", &genai.GenerateContentResponseUsageMetadata{
		PromptTokenCount:        1000,
		CachedContentTokenCount: 800,
		CandidatesTokenCount:    50,
		ThoughtsTokenCount:      120,
		TotalTokenCount:         1170,
	})

	require.NotNil(t, count)
	assert.Equal(t, int32(170), count.OutputTokens)
	select {
	case tc := <-got:
		assert.Equal(t, int32(170), tc.OutputTokens, "output includes thinking")
		assert.Equal(t, int32(200), tc.InputTokens, "input excludes cached")
		assert.Equal(t, int32(800), tc.CacheReadInputTokens)
		assert.Equal(t, int32(1170), tc.TotalTokens)
	case <-time.After(2 * time.Second):
		t.Fatal("no TokenCountEvent published")
	}
}
