package controllers

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/kcaldas/genie/cmd/events"
	"github.com/kcaldas/genie/cmd/tui/state"
	"github.com/kcaldas/genie/pkg/ai"
	core_events "github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie/genietest"
	"github.com/stretchr/testify/require"
)

var testStreamChunk = ai.StreamChunk{Text: "chunk "}

func newStreamingTestController(t *testing.T) (*ChatController, *genietest.TestFixture) {
	t.Helper()

	chatState := state.NewChatState(100)
	uiState := state.NewUIState()
	stateAccessor := state.NewStateAccessor(chatState, uiState)

	guiCommon := &mockGuiCommon{}
	component := &mockComponent{key: "test", viewName: "test"}

	fixture := genietest.NewTestFixture(t)
	fixture.StartAndGetSession()

	controller := NewChatController(
		component,
		guiCommon,
		fixture.Genie,
		stateAccessor,
		createTestConfigManager(),
		events.NewCommandEventBus(),
	)
	return controller, fixture
}

// chat.chunk and chat.response are separate topics, so their handlers
// run on separate bus goroutines. Concurrent chunk/response traffic
// must not race on the controller's streaming state (run with -race).
func TestChatControllerConcurrentChunksAndResponses(t *testing.T) {
	controller, fixture := newStreamingTestController(t)
	_ = controller

	bus := fixture.EventBus

	var wg sync.WaitGroup
	responsesSeen := make(chan struct{}, 64)
	bus.Subscribe("chat.response", func(e interface{}) {
		responsesSeen <- struct{}{}
	})

	const requests = 8
	const chunksPerRequest = 25

	wg.Add(2)
	go func() {
		defer wg.Done()
		for r := 0; r < requests; r++ {
			requestID := fmt.Sprintf("req-%d", r)
			for i := 0; i < chunksPerRequest; i++ {
				bus.Publish("chat.chunk", core_events.ChatChunkEvent{
					RequestID: requestID,
					Chunk:     &testStreamChunk,
				})
			}
		}
	}()
	go func() {
		defer wg.Done()
		for r := 0; r < requests; r++ {
			requestID := fmt.Sprintf("req-%d", r)
			bus.Publish("chat.response", core_events.ChatResponseEvent{
				RequestID: requestID,
				Response:  "final answer",
			})
		}
	}()
	wg.Wait()

	for i := 0; i < requests; i++ {
		select {
		case <-responsesSeen:
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for response %d to be delivered", i)
		}
	}
}

// A chunk arriving after its request's response must not resurrect the
// finished message as a dangling partial.
func TestChatControllerIgnoresLateChunks(t *testing.T) {
	controller, _ := newStreamingTestController(t)

	requestID := "late-req"
	controller.appendStreamingText(requestID, "partial ")
	countBefore := controller.stateAccessor.GetMessageCount()

	// Response arrives and finalizes the stream.
	buffer, ok := controller.takeStreamingMessage(requestID)
	require.True(t, ok)
	require.NotNil(t, buffer)

	// A straggler chunk for the same request must be dropped.
	controller.appendStreamingText(requestID, "stray tail")

	require.Equal(t, countBefore, controller.stateAccessor.GetMessageCount(),
		"late chunk must not create a new message")
}
