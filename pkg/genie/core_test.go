package genie

import (
	"context"
	"github.com/kcaldas/genie/pkg/ai"
	"testing"

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

func (m *MockContextManager) ChatHistory(c context.Context) ([]ctx.Message, error) {
	args := m.Called(c)
	history, _ := args.Get(0).([]ctx.Message)
	return history, args.Error(1)
}

func (m *MockContextManager) ClearContext() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockContextManager) SeedChatHistory(history []ctx.Message) {
	m.Called(history)
}

func (m *MockContextManager) RecordChatTurn(user, assistant string, activities ...events.ToolActivity) {
	m.Called(user, assistant)
}

func (m *MockContextManager) SetContextBudget(totalTokens int) {
	m.Called(totalTokens)
}

func TestPreparePromptData_PassesContextPartsThroughUntouched(t *testing.T) {
	mockCtxMgr := new(MockContextManager)
	core := &core{contextMgr: mockCtxMgr, eventBus: events.NewEventBus()}
	contextParts := map[string]string{
		"chat":    "User: Hello\nAssistant: Hi there!",
		"todo":    "- [ ] Task 1\n- [x] Task 2",
		"project": "Test project",
	}
	mockCtxMgr.On("GetContextParts", mock.Anything).Return(contextParts, nil)

	result := core.preparePromptData(context.Background(), "New message")

	assert.Equal(t, "New message", result["message"])
	assert.Equal(t, "Test project", result["project"])
	assert.Equal(t, "User: Hello\nAssistant: Hi there!", result["chat"])
	assert.Equal(t, "- [ ] Task 1\n- [x] Task 2", result["todo"], "tasks stay their own part; the layout places them")
}

// The skills system loads SKILL.md into the "active_skill" part; it must
// reach the model. Regression: it was once assembled by the provider and
// silently dropped because nothing lifted it into the prompt.
func TestBuildTurnContext_TakesItsPartsOutOfTheTemplateData(t *testing.T) {
	data := map[string]string{
		"files": " File: a.md ", "project": "# AGENTS.md", "active_skill": "# skill", "todo": "- task",
		"chat": "history", "message": "hi", "extra": "kept",
	}

	turn := buildTurnContext(data, " [Memory] ")

	assert.Equal(t, ai.TurnContext{Project: "# AGENTS.md", Files: "File: a.md", Skill: "# skill", Host: "[Memory]", Tasks: "- task"}, turn)
	assert.Equal(t, map[string]string{"chat": "history", "message": "hi", "extra": "kept"}, data)
}

func TestHistoryTurns_MapsMessagesAndActivities(t *testing.T) {
	turns := historyTurns([]ctx.Message{
		{User: "q", Activities: []events.ToolActivity{{Tool: "bash", Args: "ls", Summary: "3 files", Success: true}}, Assistant: "a"},
	})

	require.Equal(t, []ai.HistoryTurn{{User: "q", Actions: []ai.HistoryAction{{Tool: "bash", Args: "ls", Summary: "3 files"}}, Assistant: "a"}}, turns)
	assert.Nil(t, historyTurns(nil))
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
