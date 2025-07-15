package genie

import (
	"context"
	"testing"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMockPromptRunner tests the MockPromptRunner directly
func TestMockPromptRunner(t *testing.T) {
	eventBus := events.NewEventBus()
	mockRunner := NewMockPromptRunner(eventBus)

	t.Run("single expectation with event verification", func(t *testing.T) {
		// Setup expectation
		mockRunner.ExpectMessage("hello").RespondWith("Hi there!")

		// Create a mock prompt and context
		prompt := &ai.Prompt{Name: "test"}
		data := map[string]string{
			"message": "hello",
		}

		// Run the prompt
		response, err := mockRunner.RunPrompt(context.Background(), prompt, data, eventBus)
		require.NoError(t, err)
		assert.Equal(t, "Hi there!", response, "Event response should match expectation")
	})

	t.Run("multiple expectations with event verification", func(t *testing.T) {
		// Setup multiple expectations - your exact example
		mockRunner.ExpectMessage("hi").RespondWith("hello")
		mockRunner.ExpectMessage("howdy!").RespondWith("Hi mate!")

		// Test first message
		ctx1 := map[string]string{
			"message": "hi",
		}
		response1, err := mockRunner.RunPrompt(context.Background(), &ai.Prompt{Name: "test1"}, ctx1, eventBus)
		require.NoError(t, err)

		// Test second message
		ctx2 := map[string]string{
			"message": "howdy!",
		}
		response2, err := mockRunner.RunPrompt(context.Background(), &ai.Prompt{Name: "test2"}, ctx2, eventBus)
		require.NoError(t, err)

		// Verify the correct question-response mappings
		assert.Equal(t, "hello", response1, "Question 'hi' should get response 'hello'")
		assert.Equal(t, "Hi mate!", response2, "Question 'howdy!' should get response 'Hi mate!'")
	})

	t.Run("unknown message", func(t *testing.T) {
		// Try a message that wasn't expected
		ctx := map[string]string{
			"message": "unknown message",
		}
		_, err := mockRunner.RunPrompt(context.Background(), &ai.Prompt{Name: "test"}, ctx, eventBus)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no mock response configured for message")
	})

	t.Run("expectation order independence", func(t *testing.T) {
		// Setup multiple expectations - your exact example
		mockRunner.ExpectMessage("hi").RespondWith("hello")
		mockRunner.ExpectMessage("howdy!").RespondWith("Hi mate!")

		// Test second message
		ctx2 := map[string]string{
			"message": "howdy!",
		}
		response2, err := mockRunner.RunPrompt(context.Background(), &ai.Prompt{Name: "test2"}, ctx2, eventBus)
		require.NoError(t, err)

		// Test first message
		ctx1 := map[string]string{
			"message": "hi",
		}
		response1, err := mockRunner.RunPrompt(context.Background(), &ai.Prompt{Name: "test1"}, ctx1, eventBus)
		require.NoError(t, err)

		// Verify the correct question-response mappings
		assert.Equal(t, "hello", response1, "Question 'hi' should get response 'hello'")
		assert.Equal(t, "Hi mate!", response2, "Question 'howdy!' should get response 'Hi mate!'")
	})
}

