package genie_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	contextpkg "github.com/kcaldas/genie/pkg/context"
	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/genie"
	"github.com/kcaldas/genie/pkg/history"
	"github.com/kcaldas/genie/pkg/prompts"
	"github.com/kcaldas/genie/pkg/session"
	"github.com/kcaldas/genie/pkg/tools"
)

// TestGenieIntegrationWithRealDependencies tests Genie with real internal dependencies and mocked LLM
func TestGenieIntegrationWithRealDependencies(t *testing.T) {
	// Given a temporary test project directory
	testDir := createTestProject(t)
	defer os.RemoveAll(testDir)
	
	// And a real Genie instance with real internal dependencies and mocked LLM
	g, eventBus := createGenieWithRealDependencies(t, testDir)
	
	// And a way to capture chat responses
	responses := make(chan genie.ChatResponseEvent, 1)
	eventBus.Subscribe("chat.response", func(event interface{}) {
		if resp, ok := event.(genie.ChatResponseEvent); ok {
			responses <- resp
		}
	})
	
	// When I send a chat message
	sessionID := "integration-test-session"
	message := "Hello, integration test with real dependencies!"
	err := g.Chat(context.Background(), sessionID, message)
	
	// Then the chat should start without error
	if err != nil {
		t.Fatalf("Expected chat to start without error, got: %v", err)
	}
	
	// And I should eventually receive a response processed by real components
	select {
	case response := <-responses:
		if response.SessionID != sessionID {
			t.Errorf("Expected response for session %s, got %s", sessionID, response.SessionID)
		}
		if response.Message != message {
			t.Errorf("Expected response to original message %s, got %s", message, response.Message)
		}
		if response.Error != nil {
			t.Errorf("Expected successful response, got error: %v", response.Error)
		}
		if response.Response == "" {
			t.Error("Expected non-empty response")
		}
		
		// Verify the response contains our mock LLM response
		expectedResponse := "Mock LLM response to: " + message
		if response.Response != expectedResponse {
			t.Errorf("Expected response %s, got %s", expectedResponse, response.Response)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for chat response from real implementation")
	}
}

// TestGenieIntegrationSessionPersistence tests that sessions work with real session manager
func TestGenieIntegrationSessionPersistence(t *testing.T) {
	// Given a temporary test project directory
	testDir := createTestProject(t)
	defer os.RemoveAll(testDir)
	
	// And a real Genie instance
	g, _ := createGenieWithRealDependencies(t, testDir)
	
	// When I create a session
	sessionID, err := g.CreateSession()
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	
	// Then I should be able to retrieve it
	session, err := g.GetSession(sessionID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	
	if session.ID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, session.ID)
	}
}

// Use the comprehensive mock LLM client from the genie package
// (mockLLMClient implementation moved to mock_llm.go for reuse)

// createTestProject creates a temporary project directory with necessary structure
func createTestProject(t *testing.T) string {
	t.Helper()
	
	// Create temporary directory
	testDir, err := os.MkdirTemp("", "genie-test-*")
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	
	// Create .genie directory for project files
	genieDir := filepath.Join(testDir, ".genie")
	err = os.MkdirAll(genieDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .genie directory: %v", err)
	}
	
	// Create a basic conversation prompt file (needed by prompt loader)
	promptsDir := filepath.Join(testDir, "prompts")
	err = os.MkdirAll(promptsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create prompts directory: %v", err)
	}
	
	conversationPrompt := `name: conversation
instruction: "You are a helpful AI assistant."
text: "Respond to the user's message: {{.message}}"
required_tools: []
`
	
	promptFile := filepath.Join(promptsDir, "conversation.yaml")
	err = os.WriteFile(promptFile, []byte(conversationPrompt), 0644)
	if err != nil {
		t.Fatalf("Failed to create conversation prompt: %v", err)
	}
	
	return testDir
}

// createGenieWithRealDependencies creates Genie with real internal dependencies and mocked LLM
func createGenieWithRealDependencies(t *testing.T, testDir string) (genie.Genie, events.EventBus) {
	t.Helper()
	
	// Change to test directory for the duration of this test
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	
	err = os.Chdir(testDir)
	if err != nil {
		t.Fatalf("Failed to change to test directory: %v", err)
	}
	
	// Restore original directory when test completes
	t.Cleanup(func() {
		os.Chdir(originalWd)
	})
	
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
	
	// Mock only the external LLM dependency
	mockLLM := genie.NewMockLLMClient()
	mockLLM.SetDefaultResponse("Mock LLM response")
	
	// Create output formatter
	outputFormatter := tools.NewOutputFormatter(toolRegistry)
	
	// Create Genie with real internal components and mocked LLM
	deps := genie.Dependencies{
		LLMClient:       mockLLM,
		PromptLoader:    promptLoader,
		SessionMgr:      sessionMgr,
		HistoryMgr:      historyMgr,
		ContextMgr:      contextMgr,
		ChatHistoryMgr:  chatHistoryMgr,
		EventBus:        eventBus,
		OutputFormatter: outputFormatter,
	}
	
	return genie.New(deps), eventBus
}