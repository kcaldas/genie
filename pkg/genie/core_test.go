package genie

import (
	"context"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ctx"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockContextManager for testing
type MockContextManager struct {
	mock.Mock
}

func (m *MockContextManager) GetContextParts(ctx context.Context) (map[string]string, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockContextManager) ClearContext() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockContextManager) SeedChatHistory(history []ctx.Message) {
	m.Called(history)
}

func (m *MockContextManager) SetContextBudget(totalTokens int) {
	m.Called(totalTokens)
}

func TestPreparePromptData_WithTodosAndChat(t *testing.T) {
	// Setup
	mockCtxMgr := new(MockContextManager)
	eventBus := events.NewEventBus()

	core := &core{
		contextMgr: mockCtxMgr,
		eventBus:   eventBus,
	}

	// Mock context parts with both chat and todo
	contextParts := map[string]string{
		"chat":    "User: Hello\nAssistant: Hi there!",
		"todo":    "- [ ] Task 1\n- [x] Task 2",
		"project": "Test project",
	}
	mockCtxMgr.On("GetContextParts", mock.Anything).Return(contextParts, nil)

	// Execute
	result := core.preparePromptData(context.Background(), "New message")

	// Assert
	assert.Equal(t, "New message", result["message"])
	assert.Equal(t, "Test project", result["project"])

	// Check that chat was enhanced with todos
	expectedChat := "User: Hello\nAssistant: Hi there!\n\n## Current Tasks\n- [ ] Task 1\n- [x] Task 2"
	assert.Equal(t, expectedChat, result["chat"])

	// Check that todo was removed
	_, hasTodo := result["todo"]
	assert.False(t, hasTodo, "todo should be removed after being merged into chat")
}

func TestPreparePromptData_WithTodosOnly(t *testing.T) {
	// Setup
	mockCtxMgr := new(MockContextManager)
	eventBus := events.NewEventBus()

	core := &core{
		contextMgr: mockCtxMgr,
		eventBus:   eventBus,
	}

	// Mock context parts with only todos (no chat)
	contextParts := map[string]string{
		"todo":    "- [ ] Task 1\n- [x] Task 2",
		"project": "Test project",
	}
	mockCtxMgr.On("GetContextParts", mock.Anything).Return(contextParts, nil)

	// Execute
	result := core.preparePromptData(context.Background(), "New message")

	// Assert
	assert.Equal(t, "New message", result["message"])
	assert.Equal(t, "Test project", result["project"])

	// Check that chat was created with todos
	expectedChat := "## Current Tasks\n- [ ] Task 1\n- [x] Task 2"
	assert.Equal(t, expectedChat, result["chat"])

	// Check that todo was removed
	_, hasTodo := result["todo"]
	assert.False(t, hasTodo, "todo should be removed after being merged into chat")
}

func TestPreparePromptData_WithChatOnly(t *testing.T) {
	// Setup
	mockCtxMgr := new(MockContextManager)
	eventBus := events.NewEventBus()

	core := &core{
		contextMgr: mockCtxMgr,
		eventBus:   eventBus,
	}

	// Mock context parts with only chat (no todos)
	contextParts := map[string]string{
		"chat":    "User: Hello\nAssistant: Hi there!",
		"project": "Test project",
	}
	mockCtxMgr.On("GetContextParts", mock.Anything).Return(contextParts, nil)

	// Execute
	result := core.preparePromptData(context.Background(), "New message")

	// Assert
	assert.Equal(t, "New message", result["message"])
	assert.Equal(t, "Test project", result["project"])

	// Check that chat remains unchanged
	assert.Equal(t, "User: Hello\nAssistant: Hi there!", result["chat"])
}

func TestPreparePromptData_EmptyTodos(t *testing.T) {
	// Setup
	mockCtxMgr := new(MockContextManager)
	eventBus := events.NewEventBus()

	core := &core{
		contextMgr: mockCtxMgr,
		eventBus:   eventBus,
	}

	// Mock context parts with empty todo string
	contextParts := map[string]string{
		"chat":    "User: Hello\nAssistant: Hi there!",
		"todo":    "", // Empty todo content
		"project": "Test project",
	}
	mockCtxMgr.On("GetContextParts", mock.Anything).Return(contextParts, nil)

	// Execute
	result := core.preparePromptData(context.Background(), "New message")

	// Assert
	assert.Equal(t, "New message", result["message"])
	assert.Equal(t, "Test project", result["project"])

	// Check that chat remains unchanged when todos are empty
	assert.Equal(t, "User: Hello\nAssistant: Hi there!", result["chat"])

	// Check that empty todo is still present (not merged)
	assert.Equal(t, "", result["todo"])
}

func TestPreparePromptData_ContextError(t *testing.T) {
	// Setup
	mockCtxMgr := new(MockContextManager)
	eventBus := events.NewEventBus()

	core := &core{
		contextMgr: mockCtxMgr,
		eventBus:   eventBus,
	}

	// Mock context manager to return an error
	mockCtxMgr.On("GetContextParts", mock.Anything).Return((map[string]string)(nil), assert.AnError)

	// Execute
	result := core.preparePromptData(context.Background(), "New message")

	// Assert - should continue with empty context
	assert.Equal(t, "New message", result["message"])
	assert.Len(t, result, 1, "should only contain the message when context fails")
}

// TestListPersonas tests the ListPersonas method
func TestListPersonas(t *testing.T) {
	// Test case 1: ListPersonas before Start should return error
	t.Run("ListPersonas before Start", func(t *testing.T) {
		fixture := NewTestFixture(t)
		defer fixture.Cleanup()

		ctx := context.Background()
		personas, err := fixture.Genie.ListPersonas(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Genie must be started")
		assert.Nil(t, personas)
	})

	// Test case 2: ListPersonas returns internal personas
	t.Run("ListPersonas returns personas", func(t *testing.T) {
		fixture := NewTestFixture(t)
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
	fixture := NewTestFixture(t)
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
		WithImages(ChatImage{
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

func TestChatWithPromptDataMergesIntoPromptContext(t *testing.T) {
	fixture := NewTestFixture(t)
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
		WithPromptData(customData),
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
	fixture := NewTestFixture(t)
	defer fixture.Cleanup()

	fixture.StartAndGetSession(WithChatHistory(ChatHistoryTurn{User: "Earlier question", Assistant: "Earlier answer"}))

	contextMap, err := fixture.Genie.GetContext(context.Background())
	require.NoError(t, err)
	require.Contains(t, contextMap, "chat")
	assert.Contains(t, contextMap["chat"], "User: Earlier question")
	assert.Contains(t, contextMap["chat"], "Assistant: Earlier answer")
}
