package genie

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/ai"
	"github.com/kcaldas/genie/pkg/config"
	"github.com/kcaldas/genie/pkg/ctx"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/persona"
	"github.com/kcaldas/genie/pkg/prompts"
	"github.com/kcaldas/genie/pkg/session"
	"github.com/kcaldas/genie/pkg/tools"
	"github.com/stretchr/testify/require"
)

// TestFixture provides a complete testing setup for Genie with mocked dependencies
type TestFixture struct {
	Genie           Genie
	EventBus        events.EventBus
	mockLLM         *MockLLMClient   // Private - use chain-agnostic API instead
	MockChainRunner *MockChainRunner // Chain-level mocking (recommended approach)
	TestDir         string
	customChain     *ai.Chain // Allow tests to override the chain
	cleanup         func()
	t               *testing.T
	initialSession  *Session // Cache the initial session to avoid restarting
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
	projectCtxMgr := ctx.NewProjectCtxManager(eventBus)
	chatCtxMgr := ctx.NewChatCtxManager(eventBus)

	// Create registry and register providers
	registry := ctx.NewContextPartProviderRegistry()
	registry.Register(projectCtxMgr)
	registry.Register(chatCtxMgr)
	contextMgr := ctx.NewContextManager(registry)

	// Create mock LLM with sensible defaults
	mockLLM := NewMockLLMClient()
	mockLLM.SetDefaultResponse("Mock LLM response")

	// Create output formatter
	outputFormatter := tools.NewOutputFormatter(toolRegistry)

	// Create chain factory
	chainFactory := persona.NewSimpleChainFactory(promptLoader)
	
	// Create persona manager
	personaManager := persona.NewDefaultPersonaManager(chainFactory)

	// Create mock chain runner for testing
	mockChainRunner := NewMockChainRunner(eventBus)

	// Create Genie with real internal components and test AI provider
	fixture := &TestFixture{
		Genie: NewGenie(
			mockChainRunner,
			sessionMgr,
			contextMgr,
			eventBus,
			outputFormatter,
			personaManager,
			config.NewConfigManager(),
		),
		EventBus:        eventBus,
		mockLLM:         mockLLM,
		MockChainRunner: mockChainRunner,
		TestDir:         testDir,
		cleanup:         cleanup,
		t:               t,
	}

	// Apply any custom options
	for _, opt := range opts {
		opt(fixture)
	}

	return fixture
}

func WithCustomLLM(llm MockLLMClient) TestFixtureOption {
	return func(f *TestFixture) {
		f.mockLLM = &llm
	}
}

func WithRealChainProcessing() TestFixtureOption {
	return func(f *TestFixture) {
		// Create production AI provider for real chain processing
		coreInstance := f.Genie.(*core)
		handlerRegistry := ai.NewHandlerRegistry()
		chainRunner := NewDefaultChainRunner(f.mockLLM, handlerRegistry, false)

		// Rebuild Genie with production AI provider instead of test provider
		f.Genie = NewGenie(
			chainRunner,
			coreInstance.sessionMgr,
			coreInstance.contextMgr,
			f.EventBus,
			coreInstance.outputFormatter,
			coreInstance.personaManager,
			coreInstance.configMgr,
		)
		f.MockChainRunner = nil // Clear mock chain runner
	}
}

// testChainFactory implements ChainFactory for tests
type testChainFactory struct {
	chain *ai.Chain
}

func (f *testChainFactory) CreateChain(ctx context.Context) (*ai.Chain, error) {
	return f.chain, nil
}

func (f *TestFixture) UseChain(chain *ai.Chain) {
	f.customChain = chain

	// Rebuild Genie with custom chain factory
	chainFactory := &testChainFactory{chain: chain}
	personaManager := persona.NewDefaultPersonaManager(chainFactory)
	coreInstance := f.Genie.(*core)

	// Reuse the existing AI provider
	f.Genie = NewGenie(
		coreInstance.chainRunner,
		coreInstance.sessionMgr,
		coreInstance.contextMgr,
		f.EventBus,
		coreInstance.outputFormatter,
		personaManager,
		coreInstance.configMgr,
	)
}

// StartAndGetSession starts Genie and returns the initial session
// If already started, returns the cached session
func (f *TestFixture) StartAndGetSession() *Session {
	f.t.Helper()

	// Return cached session if already started
	if f.initialSession != nil {
		return f.initialSession
	}

	// Reset the Genie state to allow starting
	if coreInstance, ok := f.Genie.(*core); ok {
		coreInstance.Reset()
	}

	// Start Genie and cache the session
	session, err := f.Genie.Start(nil) // Use current directory
	require.NoError(f.t, err)
	f.initialSession = session
	return session
}

// StartChat initiates a chat and returns immediately (async operation)
func (f *TestFixture) StartChat(message string) error {
	return f.Genie.Chat(context.Background(), message)
}

// WaitForResponse waits for a chat response event with timeout
func (f *TestFixture) WaitForResponse(timeout time.Duration) *events.ChatResponseEvent {
	f.t.Helper()

	responseChan := make(chan events.ChatResponseEvent, 1)
	f.EventBus.Subscribe("chat.response", func(event interface{}) {
		if resp, ok := event.(events.ChatResponseEvent); ok {
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
func (f *TestFixture) WaitForResponseOrFail(timeout time.Duration) *events.ChatResponseEvent {
	f.t.Helper()

	response := f.WaitForResponse(timeout)
	if response == nil {
		f.t.Fatalf("Timeout waiting for chat response after %v", timeout)
	}
	return response
}

// WaitForStartedEvent waits for a chat started event with timeout
func (f *TestFixture) WaitForStartedEvent(timeout time.Duration) *events.ChatStartedEvent {
	f.t.Helper()

	startedChan := make(chan events.ChatStartedEvent, 1)
	f.EventBus.Subscribe("chat.started", func(event interface{}) {
		if started, ok := event.(events.ChatStartedEvent); ok {
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

// Cleanup manually cleans up the test fixture (normally called automatically)
func (f *TestFixture) Cleanup() {
	if f.cleanup != nil {
		f.cleanup()
	}
}

func (f *TestFixture) ExpectMessage(message string) *MockResponseBuilder {
	return f.MockChainRunner.ExpectMessage(message)
}

func (f *TestFixture) ExpectSimpleMessage(message, response string) {
	f.MockChainRunner.ExpectSimpleMessage(message, response)
}

func (f *TestFixture) ExpectMessages(responses map[string]string) {
	for input, output := range responses {
		f.MockChainRunner.ExpectSimpleMessage(input, output)
	}
}

// GetMockLLM returns the internal MockLLM for advanced testing scenarios.
// This should only be used with WithRealChainProcessing() when testing actual chain execution.
// For most tests, use the chain-agnostic ExpectMessage/ExpectSimpleMessage API instead.
func (f *TestFixture) GetMockLLM() *MockLLMClient {
	return f.mockLLM
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
