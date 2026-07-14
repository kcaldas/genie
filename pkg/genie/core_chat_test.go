package genie_test

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/genie/genietest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListPersonas tests the ListPersonas method
func TestListPersonas(t *testing.T) {
	// Test case 1: ListPersonas before Start should return error
	t.Run("ListPersonas before Start", func(t *testing.T) {
		fixture := genietest.NewTestFixture(t)
		defer fixture.Cleanup()

		ctx := context.Background()
		personas, err := fixture.Genie.ListPersonas(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Genie must be started")
		assert.Nil(t, personas)
	})

	// Test case 2: ListPersonas returns internal personas
	t.Run("ListPersonas returns personas", func(t *testing.T) {
		fixture := genietest.NewTestFixture(t)
		defer fixture.Cleanup()

		// Start Genie
		fixture.StartAndGetSession()

		ctx := context.Background()
		personas, err := fixture.Genie.ListPersonas(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, personas)
		assert.Greater(t, len(personas), 0, "Should have at least some internal personas")

		// Check that personas implement the interface correctly
		for _, p := range personas {
			assert.NotEmpty(t, p.GetID())
			assert.NotEmpty(t, p.GetName())
			assert.NotEmpty(t, p.GetSource())
		}
	})
}

func TestChatWithImagesPassesThroughToPromptRunner(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	defer fixture.Cleanup()

	fixture.StartAndGetSession()
	message := "Please describe this image"
	fixture.ExpectSimpleMessage(message, "looks great")

	responseChan := make(chan events.ChatResponseEvent, 1)
	fixture.EventBus.Subscribe("chat.response", func(evt interface{}) {
		if resp, ok := evt.(events.ChatResponseEvent); ok {
			responseChan <- resp
		}
	})

	imageBytes := []byte{0x01, 0x02, 0x03}
	err := fixture.Genie.Chat(
		context.Background(),
		message,
		genie.WithImages(genie.ChatImage{
			Data:     imageBytes,
			MIMEType: "image/jpeg",
			Filename: "sample.jpg",
		}),
	)
	require.NoError(t, err)

	select {
	case <-responseChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for chat response")
	}

	prompts := fixture.MockPromptRunner.CapturedPrompts()
	require.NotEmpty(t, prompts)
	prompt := prompts[len(prompts)-1]
	require.Len(t, prompt.Images, 1)

	img := prompt.Images[0]
	assert.Equal(t, "image/jpeg", img.Type)
	assert.Equal(t, "sample.jpg", img.Filename)
	require.Equal(t, imageBytes, img.Data)
	if len(imageBytes) > 0 {
		assert.False(t, &imageBytes[0] == &img.Data[0], "image data must be copied")
	}

	dataCaptures := fixture.MockPromptRunner.CapturedData()
	require.NotEmpty(t, dataCaptures)
	data := dataCaptures[len(dataCaptures)-1]
	assert.Equal(t, "1", data["image_count"])
}

// TestChatImagesDoNotLeakAcrossTurns guards against a regression where the
// cached persona prompt's Images slice was mutated per-turn, causing images
// sent on turn N to reappear as "fresh" attachments on turn N+1.
func TestChatImagesDoNotLeakAcrossTurns(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	defer fixture.Cleanup()

	// Use a shared, cached prompt pointer (mirrors in-memory persona flow
	// where GetPrompt returns the same *ai.Prompt on every turn).
	sharedPrompt := &ai.Prompt{Name: "test", Instruction: "be helpful"}
	fixture.UsePrompt(sharedPrompt)

	fixture.StartAndGetSession()

	turn1 := "first turn with image"
	turn2 := "second turn, text only"
	fixture.ExpectSimpleMessage(turn1, "got it")
	fixture.ExpectSimpleMessage(turn2, "ok")

	responseChan := make(chan events.ChatResponseEvent, 2)
	fixture.EventBus.Subscribe("chat.response", func(evt interface{}) {
		if resp, ok := evt.(events.ChatResponseEvent); ok {
			responseChan <- resp
		}
	})

	// Turn 1: send an image.
	err := fixture.Genie.Chat(
		context.Background(),
		turn1,
		genie.WithImages(genie.ChatImage{
			Data:     []byte{0x01, 0x02, 0x03},
			MIMEType: "image/jpeg",
			Filename: "turn1.jpg",
		}),
	)
	require.NoError(t, err)
	select {
	case <-responseChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for turn 1 response")
	}

	// Turn 2: no images at all.
	err = fixture.Genie.Chat(context.Background(), turn2)
	require.NoError(t, err)
	select {
	case <-responseChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for turn 2 response")
	}

	prompts := fixture.MockPromptRunner.CapturedPrompts()
	require.Len(t, prompts, 2)
	assert.Len(t, prompts[0].Images, 1, "turn 1 should carry its image")
	assert.Empty(t, prompts[1].Images, "turn 2 must not inherit turn 1 images")
	assert.Empty(t, sharedPrompt.Images, "cached persona prompt must not be mutated")
}

func TestChatWithPromptDataMergesIntoPromptContext(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	defer fixture.Cleanup()

	fixture.StartAndGetSession()
	message := "Please summarize the plan"
	fixture.ExpectSimpleMessage(message, "summary response")

	responseChan := make(chan events.ChatResponseEvent, 1)
	fixture.EventBus.Subscribe("chat.response", func(evt interface{}) {
		if resp, ok := evt.(events.ChatResponseEvent); ok {
			responseChan <- resp
		}
	})

	customData := map[string]string{
		"project":  "genie",
		"priority": "high",
	}

	err := fixture.Genie.Chat(
		context.Background(),
		message,
		genie.WithPromptData(customData),
	)
	require.NoError(t, err)

	select {
	case <-responseChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for chat response")
	}

	// Mutate the original map to ensure a copy was made
	customData["priority"] = "low"

	dataCaptures := fixture.MockPromptRunner.CapturedData()
	require.NotEmpty(t, dataCaptures)
	data := dataCaptures[len(dataCaptures)-1]

	assert.Equal(t, message, data["message"])
	assert.Equal(t, "genie", data["project"])
	assert.Equal(t, "high", data["priority"])
}

func TestStartWithChatHistorySeedsChatContext(t *testing.T) {
	fixture := genietest.NewTestFixture(t)
	defer fixture.Cleanup()

	fixture.StartAndGetSession(genie.WithChatHistory(genie.ChatHistoryTurn{User: "Earlier question", Assistant: "Earlier answer"}))

	contextMap, err := fixture.Genie.GetContext(context.Background())
	require.NoError(t, err)
	require.Contains(t, contextMap, "chat")
	assert.Contains(t, contextMap["chat"], "User: Earlier question")
	assert.Contains(t, contextMap["chat"], "Assistant: Earlier answer")
}
