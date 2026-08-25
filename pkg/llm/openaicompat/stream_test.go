package openaicompat

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
)

// collectStream drains an ai.Stream into its chunks.
func collectStream(t *testing.T, stream ai.Stream) []*ai.StreamChunk {
	t.Helper()
	var chunks []*ai.StreamChunk
	for {
		chunk, err := stream.Recv()
		if err != nil {
			return chunks
		}
		chunks = append(chunks, chunk)
	}
}

func TestStreamChat_EmitsTextChunks(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"role":"assistant","content":"Hello"}}]}`,
		`data: {"choices":[{"delta":{"content":" world"},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
	}, "\n")
	mock := newRawMockHTTPClient(t, func(req *http.Request) *http.Response {
		return rawResponse(http.StatusOK, body)
	})
	core := newTestCore(mock)

	stream := core.StreamChat(context.Background(), ChatRequest{Model: "m"})
	chunks := collectStream(t, stream)

	var text strings.Builder
	for _, chunk := range chunks {
		text.WriteString(chunk.Text)
	}
	assert.Equal(t, "Hello world", text.String())
}

func TestStreamChat_AccumulatesToolCallDeltasAndUsage(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"loc"}}]}}]}`,
		`data: {"choices":[{"delta":{"tool_calls":[{"id":"call_1","function":{"arguments":"ation\":\"Lisbon\"}"}}]},"finish_reason":"tool_calls"}]}`,
		`data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15,"prompt_cache_hit_tokens":4}}`,
		`data: [DONE]`,
	}, "\n")
	mock := newRawMockHTTPClient(t, func(req *http.Request) *http.Response {
		return rawResponse(http.StatusOK, body)
	})

	bus := events.NewEventBus()
	received := make(chan events.TokenCountEvent, 1)
	bus.Subscribe(events.TokenCountEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.TokenCountEvent); ok {
			received <- event
		}
	})
	core := newTestCoreWithBus(mock, bus)

	stream := core.StreamChat(context.Background(), ChatRequest{Model: "m"})
	chunks := collectStream(t, stream)

	var toolCalls []*ai.ToolCallChunk
	var tokenCount *ai.TokenCount
	for _, chunk := range chunks {
		toolCalls = append(toolCalls, chunk.ToolCalls...)
		if chunk.TokenCount != nil {
			tokenCount = chunk.TokenCount
		}
	}

	require.Len(t, toolCalls, 1)
	assert.Equal(t, "get_weather", toolCalls[0].Name)
	assert.Equal(t, map[string]any{"location": "Lisbon"}, toolCalls[0].Parameters)

	require.NotNil(t, tokenCount)
	assert.Equal(t, int32(15), tokenCount.TotalTokens)

	select {
	case event := <-received:
		assert.Equal(t, "m", event.Model)
		assert.Equal(t, int32(4), event.CachedTokens)
		assert.Equal(t, int32(6), event.InputTokens)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for token count event")
	}
}

// Streamed reasoning is aggregated into one thinking event per response.
func TestStreamChat_AggregatesReasoningIntoThinkingEvent(t *testing.T) {
	t.Parallel()

	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning_content":"step one, "}}]}`,
		`data: {"choices":[{"delta":{"reasoning_content":"step two"}}]}`,
		`data: {"choices":[{"delta":{"content":"answer"},"finish_reason":"stop"}]}`,
		`data: [DONE]`,
	}, "\n")
	mock := newRawMockHTTPClient(t, func(req *http.Request) *http.Response {
		return rawResponse(http.StatusOK, body)
	})

	bus := events.NewEventBus()
	thinking := make(chan events.ThinkingEvent, 1)
	bus.Subscribe(events.ThinkingEvent{}.Topic(), func(evt interface{}) {
		if event, ok := evt.(events.ThinkingEvent); ok {
			thinking <- event
		}
	})
	core := newTestCoreWithBus(mock, bus)

	stream := core.StreamChat(context.Background(), ChatRequest{Model: "m"})
	chunks := collectStream(t, stream)

	var text strings.Builder
	for _, chunk := range chunks {
		text.WriteString(chunk.Text)
	}
	assert.Equal(t, "answer", text.String())

	select {
	case event := <-thinking:
		assert.Equal(t, "step one, step two", event.Text)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for thinking event")
	}
}

func TestStreamChat_SurfacesStreamError(t *testing.T) {
	t.Parallel()

	body := `data: {"error":{"message":"stream exploded"}}` + "\n"
	mock := newRawMockHTTPClient(t, func(req *http.Request) *http.Response {
		return rawResponse(http.StatusOK, body)
	})
	core := newTestCore(mock)

	stream := core.StreamChat(context.Background(), ChatRequest{Model: "m"})
	_, err := stream.Recv()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream exploded")
}
