package genie_test

import (
	"testing"
	"time"

	"github.com/kcaldas/genie/pkg/genie"
)

// TestExampleTestFixture demonstrates how easy it is to write tests with the new TestFixture
func TestExampleTestFixture(t *testing.T) {
	// Before: ~80 lines of boilerplate setup (creating event bus, tools, LLM, etc.)
	// After: 1 line to get a fully configured test environment
	fixture := genie.NewTestFixture(t)

	// Create a session and send a chat message
	sessionID := fixture.CreateSession()
	err := fixture.StartChat(sessionID, "Hello from test!")
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// Wait for response with timeout
	response := fixture.WaitForResponse(2 * time.Second)
	if response == nil {
		t.Fatal("No response received")
	}

	// Verify response
	if response.Error != nil {
		t.Fatalf("Expected success, got error: %v", response.Error)
	}

	// Access the mock LLM for advanced testing
	interaction := fixture.MockLLM.GetLastInteraction()
	if interaction == nil {
		t.Fatal("No LLM interaction recorded")
	}

	t.Logf("Test completed - received response: %s", response.Response)
}