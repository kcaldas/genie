package genie_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/genie/genietest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A caller-supplied request ID must reach every lifecycle surface unchanged:
// the started event, streamed chunks (which read it from the request context),
// and the response event.
func TestChatWithRequestIDFlowsToLifecycleEvents(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	defer fixture.Cleanup()

	fixture.StartAndGetSession()
	message := "hello there"
	fixture.ExpectSimpleMessage(message, "hi")

	chunkChan := make(chan events.ChatChunkEvent, 4)
	fixture.EventBus.Subscribe("chat.chunk", func(evt interface{}) {
		if chunk, ok := evt.(events.ChatChunkEvent); ok {
			chunkChan <- chunk
		}
	})

	suppliedID := "mutiro-turn-42-attempt-1"
	err := fixture.Genie.Chat(
		context.Background(),
		message,
		genie.WithRequestID(suppliedID),
		genie.WithStreaming(true),
	)
	require.NoError(t, err)

	started := fixture.WaitForStartedEvent(2 * time.Second)
	assert.Equal(t, suppliedID, started.RequestID, "started event must carry the supplied ID")

	response := fixture.WaitForResponseOrFail(2 * time.Second)
	assert.Equal(t, suppliedID, response.RequestID, "response event must carry the supplied ID")

	select {
	case chunk := <-chunkChan:
		assert.Equal(t, suppliedID, chunk.RequestID, "chunk events must carry the supplied ID")
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for chunk event")
	}
}

// Omitting the option, or supplying a blank/whitespace-only ID, must preserve
// the existing behavior: Genie generates a UUID.
func TestChatWithBlankRequestIDGeneratesUUID(t *testing.T) {
	for name, opts := range map[string][]genie.ChatOption{
		"option omitted":  nil,
		"whitespace only": {genie.WithRequestID("   ")},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := genietest.NewTestFixture(t)
			defer fixture.Cleanup()

			fixture.StartAndGetSession()
			message := "hello"
			fixture.ExpectSimpleMessage(message, "hi")

			require.NoError(t, fixture.Genie.Chat(context.Background(), message, opts...))

			started := fixture.WaitForStartedEvent(2 * time.Second)
			_, err := uuid.Parse(started.RequestID)
			assert.NoError(t, err, "expected a generated UUID, got %q", started.RequestID)
		})
	}
}

// Two in-flight invocations must be attributable by request ID alone,
// regardless of the order their response events arrive in.
func TestChatConcurrentInvocationsCorrelateByRequestID(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	defer fixture.Cleanup()

	fixture.StartAndGetSession()
	fixture.ExpectMessages(map[string]string{
		"first question":  "first answer",
		"second question": "second answer",
	})

	responses := make(chan events.ChatResponseEvent, 2)
	fixture.EventBus.Subscribe("chat.response", func(evt interface{}) {
		if resp, ok := evt.(events.ChatResponseEvent); ok {
			responses <- resp
		}
	})

	pending := map[string]string{
		"req-first":  "first answer",
		"req-second": "second answer",
	}
	require.NoError(t, fixture.Genie.Chat(context.Background(), "first question",
		genie.WithRequestID("req-first")))
	require.NoError(t, fixture.Genie.Chat(context.Background(), "second question",
		genie.WithRequestID("req-second")))

	for range 2 {
		select {
		case resp := <-responses:
			want, known := pending[resp.RequestID]
			require.True(t, known, "response carried unknown request ID %q", resp.RequestID)
			assert.Equal(t, want, resp.Response,
				"request ID %q must select its own invocation's response", resp.RequestID)
			delete(pending, resp.RequestID)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout; still unresolved: %v", pending)
		}
	}
	assert.Empty(t, pending, "every supplied ID must be answered exactly once")
}
