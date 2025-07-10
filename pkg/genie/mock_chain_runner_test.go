package genie

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMockChainRunner tests the MockChainRunner directly
func TestMockChainRunner(t *testing.T) {
	eventBus := events.NewEventBus()
	mockRunner := NewMockChainRunner(eventBus)

	t.Run("single expectation with event verification", func(t *testing.T) {
		// Capture chat response events
		responses := make(chan events.ChatResponseEvent, 1)
		eventBus.Subscribe("chat.response", func(event interface{}) {
			if resp, ok := event.(events.ChatResponseEvent); ok {
				responses <- resp
			}
		})

		// Setup expectation
		mockRunner.ExpectMessage("hello").RespondWith("Hi there!")

		// Create a mock chain and context
		chain := &ai.Chain{Name: "test"}
		chainCtx := &ai.ChainContext{
			Data: map[string]string{
				"message": "hello",
			},
		}

		// Run the chain
		err := mockRunner.RunChain(context.Background(), chain, chainCtx, eventBus)
		require.NoError(t, err)

		// Verify the correct event was published with matching content
		select {
		case response := <-responses:
			assert.Equal(t, "hello", response.Message, "Event message should match input")
			assert.Equal(t, "Hi there!", response.Response, "Event response should match expectation")
			assert.Nil(t, response.Error, "Event should have no error")
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for chat.response event")
		}
	})

	t.Run("multiple expectations with event verification", func(t *testing.T) {
		// Capture chat response events
		responses := make(chan events.ChatResponseEvent, 2)
		eventBus.Subscribe("chat.response", func(event interface{}) {
			if resp, ok := event.(events.ChatResponseEvent); ok {
				responses <- resp
			}
		})

		// Setup multiple expectations - your exact example
		mockRunner.ExpectMessage("hi").RespondWith("hello")
		mockRunner.ExpectMessage("howdy!").RespondWith("Hi mate!")

		// Test first message
		chainCtx1 := &ai.ChainContext{
			Data: map[string]string{
				"message": "hi",
			},
		}
		err := mockRunner.RunChain(context.Background(), &ai.Chain{Name: "test1"}, chainCtx1, eventBus)
		require.NoError(t, err)

		// Test second message  
		chainCtx2 := &ai.ChainContext{
			Data: map[string]string{
				"message": "howdy!",
			},
		}
		err = mockRunner.RunChain(context.Background(), &ai.Chain{Name: "test2"}, chainCtx2, eventBus)
		require.NoError(t, err)

		// Verify both events were published with correct question-response pairs
		receivedEvents := make(map[string]string) // message -> response
		
		// Collect both events (they can arrive in any order)
		for i := 0; i < 2; i++ {
			select {
			case response := <-responses:
				assert.Nil(t, response.Error, "Event should have no error")
				receivedEvents[response.Message] = response.Response
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for chat.response events")
			}
		}

		// Verify the correct question-response mappings
		assert.Equal(t, "hello", receivedEvents["hi"], "Question 'hi' should get response 'hello'")
		assert.Equal(t, "Hi mate!", receivedEvents["howdy!"], "Question 'howdy!' should get response 'Hi mate!'")
		assert.Len(t, receivedEvents, 2, "Should receive exactly 2 events")
	})

	t.Run("unknown message", func(t *testing.T) {
		// Try a message that wasn't expected
		chainCtx := &ai.ChainContext{
			Data: map[string]string{
				"message": "unknown message",
			},
		}
		err := mockRunner.RunChain(context.Background(), &ai.Chain{Name: "test"}, chainCtx, eventBus)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no mock response configured for message")
	})

	t.Run("expectation order independence", func(t *testing.T) {
		// Setup expectations in one order
		mockRunner.ExpectMessage("first").RespondWith("response1")
		mockRunner.ExpectMessage("second").RespondWith("response2")

		// Call in reverse order
		chainCtx2 := &ai.ChainContext{
			Data: map[string]string{
				"message": "second",
			},
		}
		err := mockRunner.RunChain(context.Background(), &ai.Chain{Name: "test"}, chainCtx2, eventBus)
		require.NoError(t, err)

		chainCtx1 := &ai.ChainContext{
			Data: map[string]string{
				"message": "first",
			},
		}
		err = mockRunner.RunChain(context.Background(), &ai.Chain{Name: "test"}, chainCtx1, eventBus)
		require.NoError(t, err)
	})
}