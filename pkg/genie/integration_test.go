package genie_test

import (
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/genie"
)

// TestGenieIntegrationWithRealDependencies tests Genie with real internal dependencies and mocked LLM
func TestGenieIntegrationWithRealDependencies(t *testing.T) {
	// Given a test fixture with real dependencies and mocked LLM
	fixture := genie.NewTestFixture(t)
	
	// When I send a chat message
	sessionID := fixture.CreateSession()
	message := "Hello, integration test with real dependencies!"
	err := fixture.StartChat(sessionID, message)
	
	// Then the chat should start without error
	if err != nil {
		t.Fatalf("Expected chat to start without error, got: %v", err)
	}
	
	// And I should eventually receive a response processed by real components
	response := fixture.WaitForResponseOrFail(5 * time.Second)
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
}

// TestGenieIntegrationSessionPersistence tests that sessions work with real session manager
func TestGenieIntegrationSessionPersistence(t *testing.T) {
	// Given a test fixture with real dependencies
	fixture := genie.NewTestFixture(t)
	
	// When I create a session
	sessionID := fixture.CreateSession()
	
	// Then I should be able to retrieve it
	session := fixture.GetSession(sessionID)
	
	if session.ID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, session.ID)
	}
}

// Integration tests now use the centralized TestFixture for consistency