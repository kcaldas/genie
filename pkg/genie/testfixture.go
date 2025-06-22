package genie

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	contextpkg "github.com/kcaldas/genie/pkg/context"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/history"
	"github.com/kcaldas/genie/pkg/prompts"
	"github.com/kcaldas/genie/pkg/session"
	"github.com/kcaldas/genie/pkg/tools"
	"github.com/stretchr/testify/require"
)

// TestFixture provides a complete testing setup for Genie with mocked dependencies
type TestFixture struct {
	Genie    Genie
	EventBus events.EventBus
	MockLLM  *MockLLMClient
	TestDir  string
	cleanup  func()
	t        *testing.T
}

// TestFixtureOption allows customization of the test fixture
type TestFixtureOption func(*TestFixture)

// NewTestFixture creates a new testing fixture with real internal dependencies and mocked LLM
func NewTestFixture(t *testing.T, opts ...TestFixtureOption) *TestFixture {
	t.Helper()

	// Create temporary test directory
	testDir := createTestProject(t)

	// Change to test directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(testDir)
	require.NoError(t, err)

	// Setup cleanup function
	cleanup := func() {
		os.Chdir(originalDir)
		os.RemoveAll(testDir)
	}
	t.Cleanup(cleanup)

	// Create real internal dependencies
	eventBus := events.NewEventBus()
	toolRegistry := tools.NewDefaultRegistry(eventBus)
	promptLoader := prompts.NewPromptLoader(eventBus, toolRegistry)
	sessionMgr := session.NewSessionManager(eventBus)
	historyMgr := history.NewHistoryManager(eventBus)
	contextMgr := contextpkg.NewContextManager(eventBus)

	// Create chat history manager with test directory
	historyFilePath := filepath.Join(testDir, ".genie", "history")
	chatHistoryMgr := history.NewChatHistoryManager(historyFilePath)

	// Create mock LLM with sensible defaults
	mockLLM := NewMockLLMClient()
	mockLLM.SetDefaultResponse("Mock LLM response")

	// Create output formatter
	outputFormatter := tools.NewOutputFormatter(toolRegistry)

	// Create Genie with real internal components and mocked LLM
	deps := Dependencies{
		LLMClient:       mockLLM,
		PromptLoader:    promptLoader,
		SessionMgr:      sessionMgr,
		HistoryMgr:      historyMgr,
		ContextMgr:      contextMgr,
		ChatHistoryMgr:  chatHistoryMgr,
		EventBus:        eventBus,
		OutputFormatter: outputFormatter,
	}

	fixture := &TestFixture{
		Genie:    New(deps),
		EventBus: eventBus,
		MockLLM:  mockLLM,
		TestDir:  testDir,
		cleanup:  cleanup,
		t:        t,
	}

	// Apply any custom options
	for _, opt := range opts {
		opt(fixture)
	}

	return fixture
}

// WithCustomLLM allows replacing the mock LLM with a custom implementation
func WithCustomLLM(llm MockLLMClient) TestFixtureOption {
	return func(f *TestFixture) {
		f.MockLLM = &llm
		// Note: This requires recreating the Genie instance with new deps
		// For now, this is a placeholder - full implementation would need to rebuild
	}
}

// CreateSession creates a new session and returns the session ID
func (f *TestFixture) CreateSession() string {
	f.t.Helper()
	sessionID, err := f.Genie.CreateSession()
	require.NoError(f.t, err)
	return sessionID
}

// StartChat initiates a chat and returns immediately (async operation)
func (f *TestFixture) StartChat(sessionID, message string) error {
	return f.Genie.Chat(context.Background(), sessionID, message)
}

// WaitForResponse waits for a chat response event with timeout
func (f *TestFixture) WaitForResponse(timeout time.Duration) *ChatResponseEvent {
	f.t.Helper()
	
	responseChan := make(chan ChatResponseEvent, 1)
	f.EventBus.Subscribe("chat.response", func(event interface{}) {
		if resp, ok := event.(ChatResponseEvent); ok {
			responseChan <- resp
		}
	})

	select {
	case response := <-responseChan:
		return &response
	case <-time.After(timeout):
		return nil // Return nil on timeout, let caller handle it
	}
}

// WaitForResponseOrFail waits for a chat response and fails the test on timeout
func (f *TestFixture) WaitForResponseOrFail(timeout time.Duration) *ChatResponseEvent {
	f.t.Helper()
	
	response := f.WaitForResponse(timeout)
	if response == nil {
		f.t.Fatalf("Timeout waiting for chat response after %v", timeout)
	}
	return response
}

// WaitForStartedEvent waits for a chat started event with timeout
func (f *TestFixture) WaitForStartedEvent(timeout time.Duration) *ChatStartedEvent {
	f.t.Helper()
	
	startedChan := make(chan ChatStartedEvent, 1)
	f.EventBus.Subscribe("chat.started", func(event interface{}) {
		if started, ok := event.(ChatStartedEvent); ok {
			startedChan <- started
		}
	})

	select {
	case started := <-startedChan:
		return &started
	case <-time.After(timeout):
		f.t.Fatalf("Timeout waiting for chat started event after %v", timeout)
		return nil
	}
}

// GetSession retrieves a session by ID
func (f *TestFixture) GetSession(sessionID string) *Session {
	f.t.Helper()
	session, err := f.Genie.GetSession(sessionID)
	require.NoError(f.t, err)
	return session
}

// Cleanup manually cleans up the test fixture (normally called automatically)
func (f *TestFixture) Cleanup() {
	if f.cleanup != nil {
		f.cleanup()
	}
}

// createTestProject creates a temporary project directory with necessary structure
func createTestProject(t *testing.T) string {
	t.Helper()

	// Create temporary directory
	testDir, err := os.MkdirTemp("", "genie-testfixture-*")
	require.NoError(t, err)

	// Create .genie directory for project files
	genieDir := filepath.Join(testDir, ".genie")
	err = os.MkdirAll(genieDir, 0755)
	require.NoError(t, err)

	// Create a basic conversation prompt file (needed by prompt loader)
	promptsDir := filepath.Join(testDir, "prompts")
	err = os.MkdirAll(promptsDir, 0755)
	require.NoError(t, err)

	conversationPrompt := `name: conversation
instruction: "You are a helpful AI assistant."
text: "Respond to the user's message: {{.message}}"
required_tools: []
`

	promptFile := filepath.Join(promptsDir, "conversation.yaml")
	err = os.WriteFile(promptFile, []byte(conversationPrompt), 0644)
	require.NoError(t, err)

	return testDir
}